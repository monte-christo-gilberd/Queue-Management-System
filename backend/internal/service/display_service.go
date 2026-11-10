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
	// displayTokenBytes: jumlah byte acak untuk token layar (24 byte menjadi 32 karakter base64url).
	displayTokenBytes = 24
	// DisplayCacheTTL: umur snapshot di cache. Event membatalkannya lebih cepat; TTL hanya jaring pengaman.
	DisplayCacheTTL = time.Second

	// EventDisplayUpdated: owner mengubah, mengganti token, atau mematikan layar.
	EventDisplayUpdated = "display_updated"
)

// displayTokenLen: panjang token base64url dari 24 byte (tanpa padding).
var displayTokenLen = base64.RawURLEncoding.EncodedLen(displayTokenBytes)

// DisplayRepository: penyimpanan konfigurasi layar publik dan data antrian yang ditampilkannya. Dipenuhi
// repository (display_repo.go).
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
	// repo: penyimpanan konfigurasi layar (DisplayRepository).
	repo DisplayRepository
	// counters: daftar counter beserta nomor yang sedang dilayani untuk snapshot layar.
	counters DisplayCounters
	// guard: memastikan requester adalah owner antrian.
	guard QueueGuard
	// notifier: mengirim event display_updated. Boleh nil.
	notifier EventNotifier
	// cache: menyimpan snapshot layar agar tidak query database setiap layar meminta.
	cache *DisplayCache
}

// NewDisplayService membuat DisplayService. Bila cache nil, dibuat cache baru dengan DisplayCacheTTL.
func NewDisplayService(repo DisplayRepository, counters DisplayCounters, cache *DisplayCache) *DisplayService {
	// Jika pemanggil tidak memberi cache, buat cache baru agar layanan tetap bisa dipakai.
	if cache == nil {
		// Buat cache kosong dengan umur snapshot DisplayCacheTTL.
		cache = NewDisplayCache(DisplayCacheTTL)
	}
	return &DisplayService{repo: repo, counters: counters, cache: cache}
}

// WithQueueGuard memasang pengecekan owner. Tanpa guard, semua operasi owner gagal (fail closed).
func (s *DisplayService) WithQueueGuard(g QueueGuard) *DisplayService {
	s.guard = g
	return s
}

// WithNotifier memasang notifier untuk event DisplayUpdated, supaya layar yang terbuka tahu konfigurasinya berubah.
func (s *DisplayService) WithNotifier(n EventNotifier) *DisplayService {
	s.notifier = n
	return s
}

// owner memastikan requester adalah owner antrian. Dipakai oleh semua operasi konfigurasi layar.
func (s *DisplayService) owner(ctx context.Context, queueID, requesterID int64) error {
	// Guard belum dipasang: tolak operasi (fail closed) agar tidak ada yang lolos tanpa pengecekan.
	if s.guard == nil {
		return errNoQueueGuard
	}
	// Minta antrian dari guard; error berarti requester bukan owner atau antrian tidak ada.
	_, err := s.guard.GetQueue(ctx, queueID, requesterID)
	return err
}

// Config: konfigurasi layar; apperror.ErrNotFound bila belum diaktifkan atau bukan owner.
func (s *DisplayService) Config(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error) {
	// Pastikan requester adalah owner; jika tidak, kembalikan error ke pemanggil.
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	// Ambil konfigurasi layar antrian dari repository.
	return s.repo.Get(ctx, queueID)
}

// Configure mengaktifkan layar (token baru) atau mengubah jumlahnya (token tetap). Jumlah 0 sampai 10.
func (s *DisplayService) Configure(ctx context.Context, queueID, requesterID int64, upcoming, skipped int) (model.QueueDisplay, error) {
	// Jumlah nomor di luar rentang 0 sampai DisplayMaxCount tidak valid: tolak sebelum menyentuh database.
	if upcoming < 0 || upcoming > DisplayMaxCount || skipped < 0 || skipped > DisplayMaxCount {
		return model.QueueDisplay{}, apperror.ErrInvalidDisplayConfig
	}
	// Pastikan requester adalah owner; jika tidak, kembalikan error ke pemanggil.
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	// Buat token acak baru untuk tautan layar publik.
	token, err := newDisplayToken()
	// gagal membuat token: kembalikan error ke pemanggil.
	if err != nil {
		return model.QueueDisplay{}, err
	}
	// Simpan konfigurasi: buat baris baru jika belum ada, atau perbarui jika sudah ada (token tetap).
	d, err := s.repo.Upsert(ctx, queueID, token, upcoming, skipped)
	// gagal menyimpan: kembalikan error ke pemanggil.
	if err != nil {
		return model.QueueDisplay{}, err
	}
	// Buang cache dan kirim event agar layar yang terbuka mengambil ulang datanya.
	s.changed(queueID)
	return d, nil
}

// Rotate mengganti token: layar yang memakai token lama berhenti bekerja.
func (s *DisplayService) Rotate(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error) {
	// Pastikan requester adalah owner; jika tidak, kembalikan error ke pemanggil.
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return model.QueueDisplay{}, err
	}
	// Buat token acak baru untuk menggantikan token lama.
	token, err := newDisplayToken()
	// gagal membuat token: kembalikan error ke pemanggil.
	if err != nil {
		return model.QueueDisplay{}, err
	}
	// Simpan token baru di database; token lama tidak berlaku lagi.
	d, err := s.repo.Rotate(ctx, queueID, token)
	// gagal menyimpan: kembalikan error ke pemanggil.
	if err != nil {
		return model.QueueDisplay{}, err
	}
	// Buang cache token lama dan kirim event agar layar yang terbuka tahu tautannya sudah berganti.
	s.changed(queueID)
	return d, nil
}

// Disable mematikan layar. Idempoten.
func (s *DisplayService) Disable(ctx context.Context, queueID, requesterID int64) error {
	// Pastikan requester adalah owner; jika tidak, kembalikan error ke pemanggil.
	if err := s.owner(ctx, queueID, requesterID); err != nil {
		return err
	}
	// Hapus konfigurasi layar; jika sudah tidak ada, repository tetap dianggap berhasil (idempoten).
	if err := s.repo.Delete(ctx, queueID); err != nil {
		return err
	}
	// Buang cache dan kirim event agar layar yang terbuka tahu bahwa layar sudah dimatikan.
	s.changed(queueID)
	return nil
}

// changed membatalkan cache dan memberi sinyal ke layar yang terbuka (mereka mengambil ulang dan, bila tautan
// sudah tidak berlaku, menampilkan pesannya). Invalidate lebih dulu: Hub juga memanggilnya, tetapi tanpa hub
// (tes) cache tetap benar.
func (s *DisplayService) changed(queueID int64) {
	// Buang semua snapshot antrian ini dari cache.
	s.cache.Invalidate(queueID)
	// Kirim event DisplayUpdated ke notifier, bila notifier sudah dipasang.
	notifyEvent(s.notifier, queueID, EventDisplayUpdated, map[string]interface{}{"queue_id": queueID})
}

// validDisplayToken: bentuk token saja (tanpa database), supaya tebakan sembarang ditolak murah.
func validDisplayToken(token string) bool {
	// Panjang token harus sama dengan panjang hasil encode 24 byte; jika tidak, token pasti salah.
	if len(token) != displayTokenLen {
		return false
	}
	// Periksa setiap karakter token satu per satu.
	for i := 0; i < len(token); i++ {
		c := token[i]
		// Karakter di luar huruf, angka, '-' dan '_' tidak termasuk base64url: token ditolak.
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// QueueIDForToken: antrian pemilik token layar; apperror.ErrNotFound bila token tidak berlaku.
func (s *DisplayService) QueueIDForToken(ctx context.Context, token string) (int64, error) {
	// Bentuk token salah: tolak tanpa menyentuh cache maupun database.
	if !validDisplayToken(token) {
		return 0, apperror.ErrNotFound
	}
	// Cari di cache dulu; jika ada entri yang masih segar, pakai hasilnya tanpa query database.
	if id, ok := s.cache.QueueID(token); ok {
		return id, nil
	}
	// Cache tidak punya entri segar: cari pemilik token di database.
	d, err := s.repo.GetByToken(ctx, token)
	// gagal (termasuk token tidak ditemukan): kembalikan error ke pemanggil.
	if err != nil {
		return 0, err
	}
	return d.QueueID, nil
}

// Snapshot: keadaan antrian untuk layar publik. apperror.ErrNotFound untuk token yang tidak berlaku.
func (s *DisplayService) Snapshot(ctx context.Context, token string) (model.DisplaySnapshot, error) {
	// Bentuk token salah: tolak tanpa menyentuh cache maupun database.
	if !validDisplayToken(token) {
		return model.DisplaySnapshot{}, apperror.ErrNotFound
	}
	// Ambil snapshot dari cache; bila tidak ada entri segar, fungsi build dijalankan untuk menyusunnya dari database.
	_, snap, err := s.cache.Get(token, func() (int64, model.DisplaySnapshot, error) {
		return s.build(ctx, token)
	})
	return snap, err
}

// build menyusun snapshot layar dari database: nama, status, counter dan nomor yang dilayani, daftar berikutnya,
// dan daftar yang dilewati. Hanya dipanggil dari Snapshot lewat cache, supaya beban database tidak naik tiap layar.
func (s *DisplayService) build(ctx context.Context, token string) (int64, model.DisplaySnapshot, error) {
	// Cari konfigurasi layar dari token; ini juga memberi tahu antrian mana yang ditampilkan.
	cfg, err := s.repo.GetByToken(ctx, token)
	// gagal: kembalikan error ke pemanggil.
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	// Ambil nama, status, dan prefix nomor antrian.
	info, err := s.repo.QueueInfo(ctx, cfg.QueueID)
	// gagal: kembalikan error ke pemanggil.
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	// Ambil daftar counter beserta nomor yang sedang dilayani di tiap counter.
	counters, err := s.counters.ListWithServing(ctx, cfg.QueueID)
	// gagal: kembalikan error ke pemanggil.
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	// Ambil daftar nomor yang akan dilayani berikutnya, sebanyak jumlah yang diatur owner.
	upcoming, err := s.repo.UpcomingNumbers(ctx, cfg.QueueID, cfg.UpcomingCount)
	// gagal: kembalikan error ke pemanggil.
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}
	// Ambil daftar nomor yang dilewati, sebanyak jumlah yang diatur owner.
	skipped, err := s.repo.SkippedNumbers(ctx, cfg.QueueID, cfg.SkippedCount)
	// gagal: kembalikan error ke pemanggil.
	if err != nil {
		return 0, model.DisplaySnapshot{}, err
	}

	// Susun snapshot: label nomor dibuat dari prefix antrian, dan GeneratedAt diisi waktu UTC saat ini.
	snap := model.DisplaySnapshot{
		QueueName:    info.Name,
		Status:       info.Status,
		MultiCounter: len(counters) > 1,
		Counters:     make([]model.DisplayCounter, 0, len(counters)),
		Upcoming:     displayLabels(info.Prefix, upcoming),
		Skipped:      displayLabels(info.Prefix, skipped),
		GeneratedAt:  time.Now().UTC(),
	}
	// Deskripsi antrian hanya diisi jika ada dan tidak kosong; selain itu field dibiarkan nil.
	if info.Desc.Valid && info.Desc.String != "" {
		d := info.Desc.String
		snap.QueueDesc = &d
	}
	// Proses setiap counter untuk dimasukkan ke snapshot.
	for _, c := range counters {
		dc := model.DisplayCounter{ID: c.ID, Name: c.Name}
		// Jika counter sedang melayani nomor, beri label nomor tersebut; jika tidak, Serving dibiarkan kosong.
		if c.ServingNumber.Valid {
			dc.Serving = displayLabel(info.Prefix, c.ServingNumber.Int64)
		}
		snap.Counters = append(snap.Counters, dc)
	}
	return cfg.QueueID, snap, nil
}

// displayLabel: "A-007". Sama dengan formatTicketNumber di frontend (awalan kosong dianggap "A").
func displayLabel(prefix string, number int64) string {
	// Awalan kosong diganti "A" agar label tetap punya bentuk yang sama dengan frontend.
	if prefix == "" {
		prefix = "A"
	}
	// Format label: awalan, tanda minus, lalu nomor dengan minimal tiga digit.
	return fmt.Sprintf("%s-%03d", prefix, number)
}

// displayLabels mengubah daftar nomor menjadi label layar, dengan urutan yang sama.
func displayLabels(prefix string, numbers []int64) []string {
	out := make([]string, 0, len(numbers))
	// Proses setiap nomor dalam daftar, sesuai urutannya.
	for _, n := range numbers {
		// Ubah nomor menjadi label layar, lalu tambahkan ke daftar hasil.
		out = append(out, displayLabel(prefix, n))
	}
	return out
}

// newDisplayToken: 24 byte acak dari crypto/rand, base64url tanpa padding (32 karakter), untuk tautan layar publik.
func newDisplayToken() (string, error) {
	b := make([]byte, displayTokenBytes)
	// Isi buffer dengan byte acak dari crypto/rand; gagal berarti sumber keacakan bermasalah.
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate display token: %w", err)
	}
	// Ubah byte acak menjadi string base64url tanpa padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
