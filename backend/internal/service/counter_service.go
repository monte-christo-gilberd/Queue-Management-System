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

// CounterRepository: penyimpanan counter. Metode yang menerima tx dipakai di dalam transaksi yang mengunci
// antrian. Dipenuhi repository (counter_repo.go).
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
	// repository: penyimpanan counter (CounterRepository).
	repository CounterRepository
	// tx: menjalankan perubahan counter dalam satu transaksi.
	tx TxRunner
	// guard: memastikan requester adalah owner dan antrian masih buka.
	guard QueueGuard
	// tickets: melepas nomor yang dilayani sebelum counter dihapus. Nil = hapus counter gagal tertutup.
	tickets CounterTicketReleaser
	// notifier: mengirim event counter_updated. Boleh nil.
	notifier EventNotifier
}

// NewCounterService membuat CounterService dengan repository dan TxRunner. Notifier opsional (variadic).
func NewCounterService(
	repository CounterRepository,
	tx TxRunner,
	notifiers ...EventNotifier,
) *CounterService {
	svc := &CounterService{
		repository: repository,
		tx:         tx,
	}
	// Bila ada notifier yang diberikan, pakai yang pertama sebagai notifier service ini.
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
	// Guard belum dipasang: tolak karena pengecekan owner tidak mungkin dilakukan (fail closed).
	if s.guard == nil {
		return nil, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian lewat guard; bila gagal, kembalikan error.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return nil, err
	}
	// Ambil daftar counter beserta nomor yang sedang dilayani dari repository.
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
	// Guard belum dipasang: tolak karena pengecekan owner tidak tersedia.
	if s.guard == nil {
		return model.Counter{}, errNoQueueGuard
	}
	// Pastikan requester owner dan antrian masih buka; bila tidak, kembalikan error.
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return model.Counter{}, err
	}

	// Rapikan nama (pangkas spasi, cek panjang); nama kosong diizinkan untuk nama otomatis.
	name, err := normalizeCounterName(name, true)
	// Nama tidak valid: kembalikan error.
	if err != nil {
		return model.Counter{}, err
	}

	var created model.Counter
	// Jalankan pembuatan counter di dalam transaksi agar jumlah counter bisa dihitung dengan aman.
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Kunci baris antrian supaya pembuatan counter lain menunggu; gagal mengunci berarti error.
		if err := s.repository.LockQueue(ctx, tx, queueID); err != nil {
			return err
		}
		// Ambil daftar nama counter yang sudah ada di antrian ini.
		names, err := s.repository.Names(ctx, tx, queueID)
		// Gagal membaca daftar nama: batalkan transaksi.
		if err != nil {
			return err
		}
		// Jumlah counter sudah mencapai batas maksimum: tolak pembuatan counter baru.
		if len(names) >= MaxCounters {
			return apperror.ErrCounterLimit
		}
		// Nama kosong: beri nama otomatis; bila nama diisi tetapi sudah dipakai counter lain, tolak dengan ErrDuplicateCounterName.
		if name == "" {
			name = nextCounterName(names)
		} else if containsName(names, name) {
			// Nama yang diisi owner sudah dipakai counter lain di antrian ini: tolak.
			return apperror.ErrDuplicateCounterName
		}
		// Simpan counter baru ke repository dengan nama final.
		created, err = s.repository.Insert(ctx, tx, queueID, name)
		return err
	})
	// Transaksi gagal: kembalikan error tanpa counter.
	if err != nil {
		return model.Counter{}, err
	}

	// Beri tahu klien bahwa ada counter baru dibuat.
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
	// Guard belum dipasang: tolak karena pengecekan owner tidak tersedia.
	if s.guard == nil {
		return model.Counter{}, errNoQueueGuard
	}
	// Pastikan requester owner dan antrian masih buka; bila tidak, kembalikan error.
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return model.Counter{}, err
	}
	// Rapikan nama baru; nama kosong tidak diizinkan saat rename.
	name, err := normalizeCounterName(name, false)
	// Nama tidak valid: kembalikan error.
	if err != nil {
		return model.Counter{}, err
	}

	var renamed model.Counter
	// Jalankan pengambilan dan perubahan nama di dalam satu transaksi.
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Ambil counter yang akan diganti namanya; bila tidak ada, batalkan transaksi.
		counter, err := s.repository.Get(ctx, tx, queueID, counterID)
		// Gagal membaca counter: batalkan transaksi.
		if err != nil {
			return err
		}
		// Simpan nama baru ke repository; gagal berarti transaksi dibatalkan.
		if err := s.repository.Rename(ctx, tx, queueID, counterID, name); err != nil {
			return err
		}
		counter.Name = name
		renamed = counter
		return nil
	})
	// Transaksi gagal: kembalikan error tanpa counter.
	if err != nil {
		return model.Counter{}, err
	}

	// Beri tahu klien bahwa counter sudah diganti namanya.
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
	// Guard belum dipasang: tolak karena pengecekan owner tidak tersedia.
	if s.guard == nil {
		return errNoQueueGuard
	}
	// Releaser nomor belum dipasang: hapus counter ditolak karena nomor yang dilayani tidak bisa dilepas.
	if s.tickets == nil {
		return errNoTicketReleaser
	}
	// Pastikan requester owner dan antrian masih buka; bila tidak, kembalikan error.
	if err := s.ensureOpen(ctx, queueID, requesterID); err != nil {
		return err
	}

	// Jalankan pelepasan nomor dan penghapusan counter dalam satu transaksi.
	err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Ambil counter yang akan dihapus; gagal berarti transaksi dibatalkan.
		counter, err := s.repository.Get(ctx, tx, queueID, counterID)
		// Gagal membaca counter: batalkan transaksi.
		if err != nil {
			return err
		}
		// Counter default tidak boleh dihapus: kembalikan ErrDefaultCounter.
		if counter.IsDefault {
			return apperror.ErrDefaultCounter
		}
		// Kembalikan nomor yang sedang dilayani counter ini ke antrian; gagal berarti transaksi dibatalkan.
		if err := s.tickets.ReleaseCounterTicket(ctx, tx, queueID, counterID); err != nil {
			return err
		}
		// Hapus counter dari repository sebagai langkah terakhir transaksi.
		return s.repository.Delete(ctx, tx, queueID, counterID)
	})
	// Transaksi gagal: kembalikan error.
	if err != nil {
		return err
	}

	// Beri tahu klien bahwa counter sudah dihapus.
	s.notify(queueID, counterID, "deleted")
	return nil
}

// ensureOpen: pemilik antrian dan antrian masih buka. Non-owner: ErrNotFound; antrian ditutup: ErrQueueClosed.
func (s *CounterService) ensureOpen(
	ctx context.Context,
	queueID,
	requesterID int64,
) error {
	// Ambil antrian lewat guard; guard juga memastikan requester adalah owner.
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	// Bukan owner atau antrian tidak ada: kembalikan error.
	if err != nil {
		return err
	}
	// Antrian sudah ditutup: perubahan counter tidak boleh dilakukan.
	if queue.Status == "closed" {
		return apperror.ErrQueueClosed
	}
	return nil
}

// notify mengirim event counter_updated dengan aksi created, renamed, atau deleted.
func (s *CounterService) notify(queueID, counterID int64, action string) {
	// Kirim event counter_updated ke notifier berisi ID antrian, ID counter, dan jenis aksinya.
	notifyEvent(s.notifier, queueID, EventCounterUpdated, map[string]interface{}{
		"queue_id":   queueID,
		"counter_id": counterID,
		"action":     action,
	})
}

// normalizeCounterName memangkas spasi dan memeriksa panjang. Nama kosong diizinkan hanya bila allowEmpty
// (buat counter tanpa nama, nama otomatis).
func normalizeCounterName(name string, allowEmpty bool) (string, error) {
	// Buang spasi di awal dan akhir nama.
	name = strings.TrimSpace(name)
	// Nama kosong setelah dipangkas: diterima hanya bila allowEmpty, selain itu error.
	if name == "" {
		// Nama kosong diizinkan (nama otomatis akan dibuat nanti): kembalikan nama kosong tanpa error.
		if allowEmpty {
			return "", nil
		}
		return "", apperror.ErrInvalidCounterName
	}
	// Panjang nama dalam karakter (rune) melebihi batas: tolak dengan ErrInvalidCounterName.
	if utf8.RuneCountInString(name) > MaxCounterNameLength {
		return "", apperror.ErrInvalidCounterName
	}
	return name, nil
}

// nextCounterName: "Counter N" dengan N terkecil (mulai 2) yang belum dipakai nama lain di antrian.
func nextCounterName(names []string) string {
	// Coba N = 2, 3, 4, ... sampai ditemukan nama "Counter N" yang belum dipakai.
	for n := 2; ; n++ {
		// Bentuk kandidat nama "Counter N" untuk nilai N saat ini.
		candidate := fmt.Sprintf("Counter %d", n)
		// Nama kandidat belum dipakai counter lain: itulah nama yang dipakai.
		if !containsName(names, candidate) {
			return candidate
		}
	}
}

// containsName melaporkan apakah name sudah ada di daftar names. Pencocokan persis, jadi huruf besar dan kecil dibedakan.
func containsName(names []string, name string) bool {
	// Telusuri setiap nama yang sudah dipakai di antrian.
	for _, n := range names {
		// Nama ditemukan dalam daftar: langsung kembalikan true.
		if n == name {
			return true
		}
	}
	return false
}
