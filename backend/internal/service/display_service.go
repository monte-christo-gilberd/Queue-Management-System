package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"
	"time"
)

const (
	// DisplayMaxCount: batas jumlah nomor "berikutnya" dan "dilewati" di layar.
	DisplayMaxCount = 10
	// DisplayDefaultUpcoming dipakai saat layar pertama kali diaktifkan tanpa jumlah.
	DisplayDefaultUpcoming = 5
	displayTokenBytes      = 24
	// DisplayCacheTTL: umur snapshot di cache. Event membatalkannya lebih cepat; TTL hanya jaring pengaman.
	DisplayCacheTTL = time.Second

	// EventDisplayUpdated: owner mengubah, mengganti token, atau mematikan layar.
	EventDisplayUpdated = "display_updated"
)

// displayTokenLen: panjang token base64url dari 24 byte (tanpa padding).
var displayTokenLen = base64.RawURLEncoding.EncodedLen(displayTokenBytes)

type DisplayRepository interface {
	Get(ctx context.Context, queueID int64) (model.QueueDisplay, error)
	GetByToken(ctx context.Context, token string) (model.QueueDisplay, error)
	Upsert(ctx context.Context, queueID int64, token string, upcoming, skipped int) (model.QueueDisplay, error)
	Rotate(ctx context.Context, queueID int64, token string) (model.QueueDisplay, error)
	Delete(ctx context.Context, queueID int64) error
	QueueInfo(ctx context.Context, queueID int64) (model.DisplayQueueInfo, error)
	UpcomingNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error)
	SkippedNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error)
}

// DisplayCounters: daftar counter beserta nomor yang dilayani (CounterRepository.ListWithServing).
type DisplayCounters interface {
	ListWithServing(ctx context.Context, queueID int64) ([]model.CounterWithServing, error)
}

// DisplayService: konfigurasi layar publik (hanya owner antrian) dan snapshot publik (siapa pun yang punya token).
type DisplayService struct {
	repo     DisplayRepository
	counters DisplayCounters
	guard    QueueGuard
	notifier EventNotifier
	cache    *DisplayCache
}

func NewDisplayService(repo DisplayRepository, counters DisplayCounters, cache *DisplayCache) *DisplayService {
	if cache == nil {
		cache = NewDisplayCache(DisplayCacheTTL)
	}
	return &DisplayService{repo: repo, counters: counters, cache: cache}
}

// WithQueueGuard memasang pengecekan owner. Tanpa guard, semua operasi owner gagal (fail closed).
func (s *DisplayService) WithQueueGuard(g QueueGuard) *DisplayService {
	s.guard = g
	return s
}

func (s *DisplayService) WithNotifier(n EventNotifier) *DisplayService {
	s.notifier = n
	return s
}

func (s *DisplayService) owner(ctx context.Context, queueID, requesterID int64) error {
	if s.guard == nil {
		return errNoQueueGuard
	}
	_, err := s.guard.GetQueue(ctx, queueID, requesterID)
	return err
}

// Config: konfigurasi layar; apperror.ErrNotFound bila belum diaktifkan atau bukan owner.
func (s *DisplayService) Config(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error) {
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	return s.repo.Get(ctx, queueID)
}

// Configure mengaktifkan layar (token baru) atau mengubah jumlahnya (token tetap). Jumlah 0 sampai 10.
func (s *DisplayService) Configure(ctx context.Context, queueID, requesterID int64, upcoming, skipped int) (model.QueueDisplay, error) {
	if upcoming < 0 || upcoming > DisplayMaxCount || skipped < 0 || skipped > DisplayMaxCount {
		return model.QueueDisplay{}, apperror.ErrInvalidDisplayConfig
	}
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	token, err := newDisplayToken()
	if err != nil {
		return model.QueueDisplay{}, err
	}
	d, err := s.repo.Upsert(ctx, queueID, token, upcoming, skipped)
	if err != nil {
		return model.QueueDisplay{}, err
	}
	s.changed(queueID)
	return d, nil
}

// Rotate mengganti token: layar yang memakai token lama berhenti bekerja.
func (s *DisplayService) Rotate(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error) {
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	token, err := newDisplayToken()
	if err != nil {
		return model.QueueDisplay{}, err
	}
	d, err := s.repo.Rotate(ctx, queueID, token)
	if err != nil {
		return model.QueueDisplay{}, err
	}
	s.changed(queueID)
	return d, nil
}

// Disable mematikan layar. Idempoten.
func (s *DisplayService) Disable(ctx context.Context, queueID, requesterID int64) error {
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, queueID); err != nil {
		return err
	}
	s.changed(queueID)
	return nil
}

// changed membatalkan cache dan memberi sinyal ke layar yang terbuka (mereka mengambil ulang dan, bila tautan
// sudah tidak berlaku, menampilkan pesannya). Invalidate lebih dulu: Hub juga memanggilnya, tetapi tanpa hub
// (tes) cache tetap benar.
func (s *DisplayService) changed(queueID int64) {
	s.cache.Invalidate(queueID)
	notifyEvent(s.notifier, queueID, EventDisplayUpdated, map[string]interface{}{"queue_id": queueID})
}

// validDisplayToken: bentuk token saja (tanpa database), supaya tebakan sembarang ditolak murah.
func validDisplayToken(token string) bool {
	if len(token) != displayTokenLen {
		return false
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// QueueIDForToken: antrian pemilik token layar; apperror.ErrNotFound bila token tidak berlaku.
func (s *DisplayService) QueueIDForToken(ctx context.Context, token string) (int64, error) {
	if !validDisplayToken(token) {
		return 0, apperror.ErrNotFound
	}
	if id, ok := s.cache.QueueID(token); ok {
		return id, nil
	}
	d, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return 0, err
	}
	return d.QueueID, nil
}

// Snapshot: keadaan antrian untuk layar publik. apperror.ErrNotFound untuk token yang tidak berlaku.
func (s *DisplayService) Snapshot(ctx context.Context, token string) (model.DisplaySnapshot, error) {
	if !validDisplayToken(token) {
		return model.DisplaySnapshot{}, apperror.ErrNotFound
	}
	_, snap, err := s.cache.Get(token, func() (int64, model.DisplaySnapshot, error) {
		return s.build(ctx, token)
	})
	return snap, err
}

func (s *DisplayService) build(ctx context.Context, token string) (int64, model.DisplaySnapshot, error) {
	cfg, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	info, err := s.repo.QueueInfo(ctx, cfg.QueueID)
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	counters, err := s.counters.ListWithServing(ctx, cfg.QueueID)
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	upcoming, err := s.repo.UpcomingNumbers(ctx, cfg.QueueID, cfg.UpcomingCount)
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	skipped, err := s.repo.SkippedNumbers(ctx, cfg.QueueID, cfg.SkippedCount)
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}

	snap := model.DisplaySnapshot{
		QueueName:    info.Name,
		Status:       info.Status,
		MultiCounter: len(counters) > 1,
		Counters:     make([]model.DisplayCounter, 0, len(counters)),
		Upcoming:     displayLabels(info.Prefix, upcoming),
		Skipped:      displayLabels(info.Prefix, skipped),
		GeneratedAt:  time.Now().UTC(),
	}
	if info.Desc.Valid && info.Desc.String != "" {
		d := info.Desc.String
		snap.QueueDesc = &d
	}
	for _, c := range counters {
		dc := model.DisplayCounter{ID: c.ID, Name: c.Name}
		if c.ServingNumber.Valid {
			dc.Serving = displayLabel(info.Prefix, c.ServingNumber.Int64)
		}
		snap.Counters = append(snap.Counters, dc)
	}
	return cfg.QueueID, snap, nil
}

// displayLabel: "A-007". Sama dengan formatTicketNumber di frontend (awalan kosong dianggap "A").
func displayLabel(prefix string, number int64) string {
	if prefix == "" {
		prefix = "A"
	}
	return fmt.Sprintf("%s-%03d", prefix, number)
}

func displayLabels(prefix string, numbers []int64) []string {
	out := make([]string, 0, len(numbers))
	for _, n := range numbers {
		out = append(out, displayLabel(prefix, n))
	}
	return out
}

func newDisplayToken() (string, error) {
	b := make([]byte, displayTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate display token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
