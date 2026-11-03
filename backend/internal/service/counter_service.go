package service

import (
	"context"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"
	"strings"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
)

// MaxCounters: batas jumlah counter per antrian, termasuk counter default (D3).
const MaxCounters = 10

// MaxCounterNameLength: panjang nama counter dalam karakter (rune), setelah dipangkas (D3).
const MaxCounterNameLength = 30

type CounterRepository interface {
	ListWithServing(
		ctx context.Context,
		queueID int64,
	) ([]model.CounterWithServing, error)
	LockQueue(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) error
	Names(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) ([]string, error)
	Insert(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
		name string,
	) (model.Counter, error)
	Get(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) (model.Counter, error)
	Rename(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
		name string,
	) error
	Delete(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) error
}

// CounterTicketReleaser melepas nomor yang sedang dilayani sebelum counter dihapus. Dipenuhi TicketService.
type CounterTicketReleaser interface {
	ReleaseCounterTicket(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) error
}

// CounterService: CRUD counter antrian (4.11). Hanya owner. Antrian yang sudah ditutup tidak bisa diubah (D6).
type CounterService struct {
	repository CounterRepository
	tx         TxRunner
	guard      QueueGuard
	tickets    CounterTicketReleaser
	notifier   EventNotifier
}

func NewCounterService(
	repository CounterRepository,
	tx TxRunner,
	notifiers ...EventNotifier,
) *CounterService {
	svc := &CounterService{
		repository: repository,
		tx:         tx,
	}
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithQueueGuard memasang pengecekan owner. Tanpa guard, semua operasi gagal (fail closed).
func (s *CounterService) WithQueueGuard(
	guard QueueGuard,
) *CounterService {
	s.guard = guard
	return s
}

// WithTicketReleaser memasang pelepasan nomor saat counter dihapus. Tanpa ini, hapus counter gagal tertutup.
func (s *CounterService) WithTicketReleaser(
	tickets CounterTicketReleaser,
) *CounterService {
	s.tickets = tickets
	return s
}

// List: counter antrian beserta nomor yang sedang dilayani masing-masing.
func (s *CounterService) List(
	ctx context.Context,
	queueID,
	requesterID int64,
) ([]model.CounterWithServing, error) {
	if s.guard == nil {
		return nil, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return nil, err
	}
	return s.repository.ListWithServing(ctx, queueID)
}

// Create membuat counter. Nama kosong = nama otomatis "Counter N" (N terkecil yang belum dipakai, mulai dari 2),
// supaya membuat tab baru cukup satu ketukan. Batas MaxCounters dihitung di dalam transaksi yang mengunci antrian.
func (s *CounterService) Create(
	ctx context.Context,
	queueID,
	requesterID int64,
	name string,
) (model.Counter, error) {
	if s.guard == nil {
		return model.Counter{}, errNoQueueGuard
	}
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return model.Counter{}, err
	}

	name, err := normalizeCounterName(name, true)
	if err != nil {
		return model.Counter{}, err
	}

	var created model.Counter
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := s.repository.LockQueue(ctx, tx, queueID); err != nil {
			return err
		}
		names, err := s.repository.Names(ctx, tx, queueID)
		if err != nil {
			return err
		}
		if len(names) >= MaxCounters {
			return apperror.ErrCounterLimit
		}
		if name == "" {
			name = nextCounterName(names)
		} else if containsName(names, name) {
			return apperror.ErrDuplicateCounterName
		}
		created, err = s.repository.Insert(ctx, tx, queueID, name)
		return err
	})
	if err != nil {
		return model.Counter{}, err
	}

	s.notify(queueID, created.ID, "created")
	return created, nil
}

// Rename mengganti nama counter, termasuk counter default (D1).
func (s *CounterService) Rename(
	ctx context.Context,
	queueID,
	requesterID,
	counterID int64,
	name string,
) (model.Counter, error) {
	if s.guard == nil {
		return model.Counter{}, errNoQueueGuard
	}
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return model.Counter{}, err
	}
	name, err := normalizeCounterName(name, false)
	if err != nil {
		return model.Counter{}, err
	}

	var renamed model.Counter
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		counter, err := s.repository.Get(ctx, tx, queueID, counterID)
		if err != nil {
			return err
		}
		if err := s.repository.Rename(ctx, tx, queueID, counterID, name); err != nil {
			return err
		}
		counter.Name = name
		renamed = counter
		return nil
	})
	if err != nil {
		return model.Counter{}, err
	}

	s.notify(queueID, counterID, "renamed")
	return renamed, nil
}

// Delete menghapus counter. Counter default tidak bisa dihapus (D1). Nomor yang sedang dilayani counter ini
// kembali ke posisi 1 urutan tunggu (D2). Semuanya dalam satu transaksi.
func (s *CounterService) Delete(
	ctx context.Context,
	queueID,
	requesterID,
	counterID int64,
) error {
	if s.guard == nil {
		return errNoQueueGuard
	}
	if s.tickets == nil {
		return errNoTicketReleaser
	}
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return err
	}

	err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		counter, err := s.repository.Get(ctx, tx, queueID, counterID)
		if err != nil {
			return err
		}
		if counter.IsDefault {
			return apperror.ErrDefaultCounter
		}
		if err := s.tickets.ReleaseCounterTicket(ctx, tx, queueID, counterID); err != nil {
			return err
		}
		return s.repository.Delete(ctx, tx, queueID, counterID)
	})
	if err != nil {
		return err
	}

	s.notify(queueID, counterID, "deleted")
	return nil
}

// ensureOpen: pemilik antrian dan antrian masih buka. Non-owner: ErrNotFound; antrian ditutup: ErrQueueClosed.
func (s *CounterService) ensureOpen(
	ctx context.Context,
	queueID,
	requesterID int64,
) error {
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	if err != nil {
		return err
	}
	if queue.Status == "closed" {
		return apperror.ErrQueueClosed
	}
	return nil
}

func (s *CounterService) notify(queueID, counterID int64, action string) {
	notifyEvent(s.notifier, queueID, EventCounterUpdated, map[string]interface{}{
		"queue_id":   queueID,
		"counter_id": counterID,
		"action":     action,
	})
}

// normalizeCounterName memangkas spasi dan memeriksa panjang. Nama kosong diizinkan hanya bila allowEmpty
// (buat counter tanpa nama, nama otomatis).
func normalizeCounterName(name string, allowEmpty bool) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		if allowEmpty {
			return "", nil
		}
		return "", apperror.ErrInvalidCounterName
	}
	if utf8.RuneCountInString(name) > MaxCounterNameLength {
		return "", apperror.ErrInvalidCounterName
	}
	return name, nil
}

// nextCounterName: "Counter N" dengan N terkecil (mulai 2) yang belum dipakai nama lain di antrian.
func nextCounterName(names []string) string {
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("Counter %d", n)
		if !containsName(names, candidate) {
			return candidate
		}
	}
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
