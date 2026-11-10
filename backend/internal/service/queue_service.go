package service

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

// QueueRepository: penyimpanan antrian, pengaturan skip, QR, dan counter default. Dipenuhi repository
// (queue_repo.go).
type QueueRepository interface {
	CreateNewQueue(
		ctx context.Context,
		tx *sqlx.Tx,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, error)
	GetQueueByID(
		ctx context.Context,
		queueID int64,
	) (model.Queue, error)
	ListQueueByOwner(
		ctx context.Context,
		ownerID int64,
		activeOnly bool,
	) ([]model.Queue, error)
	SetTicketPrefix(
		ctx context.Context,
		queueID int64,
		prefix string,
	) error
	UpdateQueueDetails(
		ctx context.Context,
		queueID int64,
		queueName,
		queueDescription string,
	) error
	CloseQueue(
		ctx context.Context,
		queueID int64,
	) error
	GetQueueSettings(
		ctx context.Context,
		queueID int64,
	) (model.QueueSettings, error)
	CreateQueueSettingsIfMissing(
		ctx context.Context,
		queueID int64,
	) error
	GetSkipSettings(
		ctx context.Context,
		queueID int64,
	) (model.SkipSettings, error)
	UpdateSkipSettings(
		ctx context.Context,
		queueID int64,
		settings model.SkipSettings,
	) error
	UpdateQRType(
		ctx context.Context,
		queueID int64,
		qrType string,
	) error
	UpdateQRCurrentCode(
		ctx context.Context,
		queueID int64,
		token string,
	) error
	CreateDefaultCounter(
		ctx context.Context,
		queueID int64,
		name string,
	) (model.Counter, error)
	GetCounterByQueue(
		ctx context.Context,
		queueID int64,
	) (model.Counter, error)
}

// SubscriptionChecker: mengecek apakah owner punya langganan aktif. Dipenuhi repository (owner_repo.go).
type SubscriptionChecker interface {
	IsActive(
		ctx context.Context,
		ownerID int64,
	) (bool, error)
}

// TicketHolderChecker melaporkan apakah user pernah punya tiket di sebuah queue, dalam status apa pun
// (termasuk yang sudah selesai), supaya pemegang tiket tetap bisa membaca queue. Dipenuhi TicketRepository.
type TicketHolderChecker interface {
	HasAnyTicket(
		ctx context.Context,
		queueID,
		userID int64,
	) (bool, error)
}

// QueueService: antrian (buat, ubah, tutup, prefix nomor, pengaturan skip) dan pengecekan kepemilikan.
// Dependency yang tidak wajib dipasang lewat With... (lihat WithTicketHolderChecker, WithVerificationGuard).
type QueueService struct {
	repository   QueueRepository
	subscription SubscriptionChecker
	tx           TxRunner
	notifier     EventNotifier
	holders      TicketHolderChecker
	verification VerificationGuard
}

// NewQueueService membuat QueueService dengan repository, pengecek langganan, dan TxRunner. Notifier opsional
// (variadic): hanya notifiers[0] yang dipakai.
func NewQueueService(
	repository QueueRepository,
	subscription SubscriptionChecker,
	tx TxRunner,
	notifiers ...EventNotifier,
) *QueueService {
	svc := &QueueService{
		repository:   repository,
		subscription: subscription,
		tx:           tx,
	}
	// Bila ada notifier yang diberikan, pakai yang pertama sebagai notifier service ini.
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithTicketHolderChecker mengaktifkan akses baca queue untuk pemegang tiket (lihat ViewQueue).
func (s *QueueService) WithTicketHolderChecker(
	holders TicketHolderChecker,
) *QueueService {
	s.holders = holders
	return s
}

// WithVerificationGuard mewajibkan email terverifikasi untuk membuat antrian (apperror.ErrEmailNotVerified).
// Tanpa guard (verifikasi email mati), tidak ada pemeriksaan.
func (s *QueueService) WithVerificationGuard(
	guard VerificationGuard,
) *QueueService {
	s.verification = guard
	return s
}

// CreateQueue membuat antrian baru beserta counter default. Urutan pemeriksaan: email harus terverifikasi
// (bila verifikasi dinyalakan), lalu langganan owner harus aktif.
func (s *QueueService) CreateQueue(
	ctx context.Context,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, error) {

	// Verifikasi email diperiksa lebih dulu daripada langganan: user yang belum terverifikasi tidak perlu
	// diminta membayar sebelum tahu bahwa emailnya harus diverifikasi.
	if s.verification != nil {
		// Minta guard memastikan email owner sudah terverifikasi; bila belum, kembalikan error ke pemanggil.
		if err := s.verification.RequireVerified(ctx, ownerID); err != nil {
			return model.Queue{}, err
		}
	}

	// Pastikan langganan owner aktif sebelum antrian dibuat.
	err := s.isSubscribed(
		ctx,
		ownerID,
	)
	// Gagal cek langganan (langganan tidak aktif atau error lain): hentikan proses dan kembalikan error.
	if err != nil {
		return model.Queue{}, err
	}

	var queue model.Queue
	// Jalankan pembuatan antrian di dalam transaksi supaya antrian dan counter default tersimpan bersama.
	err = s.tx.WithTx(
		ctx,
		func(tx *sqlx.Tx) error {
			var err error
			// Simpan antrian baru lewat repository di dalam transaksi ini.
			queue, err = s.repository.CreateNewQueue(
				ctx,
				tx,
				ownerID,
				queueName,
				queueDescription,
			)
			// Gagal menyimpan antrian: batalkan transaksi dengan mengembalikan error.
			if err != nil {
				return err
			}
			return nil
		},
	)

	// Transaksi gagal: kembalikan error ke pemanggil tanpa antrian.
	if err != nil {
		return model.Queue{}, err
	}
	return queue, nil
}

// SetTicketPrefix mengubah prefix nomor tiket (owner antrian, langganan aktif). Prefix sudah dinormalisasi handler.
func (s *QueueService) SetTicketPrefix(
	ctx context.Context,
	queueID,
	requesterID int64,
	prefix string,
) error {
	// Pastikan langganan requester aktif; bila tidak, kembalikan error.
	if err := s.isSubscribed(ctx, requesterID); err != nil {
		return err
	}
	// Pastikan requester adalah owner antrian; bila bukan, kembalikan error.
	if _, err := s.validateOwnership(ctx, queueID, requesterID); err != nil {
		return err
	}
	// Simpan prefix nomor tiket yang baru ke antrian.
	if err := s.repository.SetTicketPrefix(ctx, queueID, prefix); err != nil {
		return err
	}
	// Kirim event queue_updated agar klien yang terhubung tahu prefix sudah berubah.
	notifyEvent(s.notifier, queueID, EventQueueUpdated, map[string]interface{}{"queue_id": queueID})
	return nil
}

// CreateDefaultCounter membuat counter bernama tertentu; dipakai bila queue
// lama (dibuat sebelum counter default otomatis) belum punya counter.
func (s *QueueService) CreateDefaultCounter(
	ctx context.Context,
	queueID int64,
	name string,
) (model.Counter, error) {
	// Teruskan pembuatan counter default ke repository dan kembalikan hasilnya apa adanya.
	return s.repository.CreateDefaultCounter(ctx, queueID, name)
}

// GetCounterByQueue mengambil counter default milik queue (is_default atau satu saja).
func (s *QueueService) GetCounterByQueue(
	ctx context.Context,
	queueID int64,
) (model.Counter, error) {
	// Ambil counter milik antrian dari repository dan kembalikan hasilnya.
	return s.repository.GetCounterByQueue(ctx, queueID)
}

// GetQueue mengembalikan antrian hanya untuk owner-nya. Dipakai sebagai QueueGuard oleh service lain.
// Selain owner: ErrNotFound.
func (s *QueueService) GetQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, error) {

	// Ambil antrian sekaligus pastikan requester adalah owner-nya.
	queue, err := s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	// Bukan owner atau antrian tidak ditemukan: kembalikan error.
	if err != nil {
		return model.Queue{}, err
	}

	return queue, nil
}

// ViewQueue: owner mendapat queue (isOwner true); pemegang tiket mendapat queue saja
// (isOwner false); selain itu apperror.ErrNotFound.
func (s *QueueService) ViewQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, bool, error) {

	// Ambil data antrian dari repository.
	queue, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	// Antrian tidak bisa dibaca: kembalikan error.
	if err != nil {
		return model.Queue{}, false, err
	}

	// Requester adalah owner antrian: kembalikan antrian dengan isOwner true.
	if requesterID == queue.OwnerID {
		return queue, true, nil
	}

	// Cek pemegang tiket hanya bila checker dipasang (lihat WithTicketHolderChecker).
	if s.holders != nil {
		// Periksa apakah requester pernah punya tiket di antrian ini, dalam status apa pun.
		isHolder, err := s.holders.HasAnyTicket(
			ctx,
			queueID,
			requesterID,
		)
		// Pengecekan tiket gagal: kembalikan error.
		if err != nil {
			return model.Queue{}, false, err
		}
		// Requester pemegang tiket: boleh membaca antrian, tetapi bukan owner.
		if isHolder {
			return queue, false, nil
		}
	}

	return model.Queue{}, false, apperror.ErrNotFound
}

// GetActiveQueue memastikan queue ada (ErrNotFound) dan masih aktif (ErrQueueClosed).
func (s *QueueService) GetActiveQueue(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {

	// Ambil antrian dari repository untuk dicek statusnya.
	queue, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	// Antrian tidak ditemukan atau gagal dibaca: kembalikan error.
	if err != nil {
		return model.Queue{}, err
	}
	// Status antrian bukan "active" (sudah ditutup): kembalikan ErrQueueClosed.
	if queue.Status != "active" {
		return model.Queue{}, apperror.ErrQueueClosed
	}

	return queue, nil
}

// ListQueue mengembalikan daftar antrian milik requester. Tanpa langganan aktif, hanya antrian yang masih
// aktif yang dikembalikan.
func (s *QueueService) ListQueue(
	ctx context.Context,
	requesterID int64,
) ([]model.Queue, error) {

	// Cek langganan requester; hasilnya (nil bila aktif, error bila tidak) menentukan filter di bawah.
	err := s.isSubscribed(
		ctx,
		requesterID,
	)

	activeOnly := true
	// Langganan aktif (err nil): tampilkan semua antrian, termasuk yang sudah ditutup.
	if err == nil {
		activeOnly = false
	}

	// Ambil daftar antrian milik requester sesuai filter activeOnly.
	queues, err := s.repository.ListQueueByOwner(
		ctx,
		requesterID,
		activeOnly,
	)
	// Gagal mengambil daftar: kembalikan slice kosong beserta error.
	if err != nil {
		return []model.Queue{}, err
	}

	return queues, nil
}

// UpdateQueue mengubah nama dan deskripsi antrian (owner dengan langganan aktif), lalu mengirim event queue_updated.
func (s *QueueService) UpdateQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
	queueName,
	queueDescription string,
) error {

	// Pastikan langganan requester aktif sebelum mengubah antrian.
	err := s.isSubscribed(
		ctx,
		requesterID,
	)
	// Langganan tidak aktif atau error: hentikan proses.
	if err != nil {
		return err
	}

	// Pastikan requester adalah owner antrian.
	_, err = s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	// Bukan owner atau antrian tidak ada: hentikan proses.
	if err != nil {
		return err
	}

	// Simpan nama dan deskripsi baru ke repository.
	err = s.repository.UpdateQueueDetails(
		ctx,
		queueID,
		queueName,
		queueDescription,
	)

	// Penyimpanan gagal: kembalikan error tanpa mengirim event.
	if err != nil {
		return err
	}
	// Beri tahu klien bahwa antrian sudah berubah.
	notifyEvent(
		s.notifier,
		queueID,
		EventQueueUpdated,
		map[string]interface{}{
			"queue_id": queueID,
		},
	)
	return nil
}

// GetSkipSettings: pengaturan skip antrian (hanya owner).
func (s *QueueService) GetSkipSettings(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.SkipSettings, error) {
	// Pastikan requester adalah owner; bila bukan, kembalikan error.
	if _, err := s.validateOwnership(ctx, queueID, requesterID); err != nil {
		return model.SkipSettings{}, err
	}
	// Ambil pengaturan skip antrian dari repository.
	return s.repository.GetSkipSettings(ctx, queueID)
}

// UpdateSkipSettings menyimpan pengaturan skip. Mode harus dikenal, N >= 1 untuk after_n, batas kembali >= 1.
func (s *QueueService) UpdateSkipSettings(
	ctx context.Context,
	queueID,
	requesterID int64,
	settings model.SkipSettings,
) (model.SkipSettings, error) {
	// Pastikan langganan requester aktif.
	if err := s.isSubscribed(ctx, requesterID); err != nil {
		return model.SkipSettings{}, err
	}
	// Pastikan requester adalah owner dan ambil data antrian untuk dicek statusnya.
	queue, err := s.validateOwnership(ctx, queueID, requesterID)
	// Bukan owner atau antrian tidak ada: hentikan proses.
	if err != nil {
		return model.SkipSettings{}, err
	}
	// Antrian sudah ditutup: pengaturan skip tidak boleh diubah lagi.
	if queue.Status == "closed" {
		return model.SkipSettings{}, apperror.ErrQueueClosed
	}

	// Periksa mode skip yang dipilih dan terapkan aturan untuk tiap mode.
	switch settings.Fallback {
	// Mode remove atau back tidak memakai N, jadi N dikosongkan (0).
	case model.SkipFallbackRemove, model.SkipFallbackBack:
		settings.N = 0
	// Mode after_n wajib punya N minimal 1.
	case model.SkipFallbackAfterN:
		// N kurang dari 1 untuk mode after_n tidak valid.
		if settings.N < 1 {
			return model.SkipSettings{}, apperror.ErrInvalidSkipSettings
		}
	// Mode tidak dikenal: tolak dengan ErrInvalidSkipSettings.
	default:
		return model.SkipSettings{}, apperror.ErrInvalidSkipSettings
	}
	// Batas kembali harus minimal 1; nilai lebih kecil tidak valid.
	if settings.MaxReturns < 1 {
		return model.SkipSettings{}, apperror.ErrInvalidSkipSettings
	}

	// Simpan pengaturan skip baru; bila gagal, kembalikan error.
	if err := s.repository.UpdateSkipSettings(ctx, queueID, settings); err != nil {
		return model.SkipSettings{}, err
	}
	return settings, nil
}

// CloseQueue menutup antrian (hanya owner). Setelah ditutup, antrian tidak menerima tiket baru dan
// tidak bisa diubah lagi; event queue_closed dikirim.
func (s *QueueService) CloseQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) error {

	// Pastikan requester adalah owner antrian.
	_, err := s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	// Bukan owner atau antrian tidak ada: hentikan proses.
	if err != nil {
		return err
	}

	// Tandai antrian sebagai ditutup di repository.
	err = s.repository.CloseQueue(
		ctx,
		queueID,
	)
	// Gagal menutup antrian: kembalikan error tanpa mengirim event.
	if err != nil {
		return err
	}

	// Beri tahu klien bahwa antrian sudah ditutup.
	notifyEvent(
		s.notifier,
		queueID,
		EventQueueClosed,
		map[string]interface{}{
			"queue_id": queueID,
		},
	)

	return nil
}

// isSubscribed memastikan langganan owner aktif; bila tidak, ErrSubscriptionInactive.
func (s *QueueService) isSubscribed(
	ctx context.Context,
	ownerID int64,
) error {

	// Tanyakan ke pengecek langganan apakah owner punya langganan aktif.
	isSubscribed, err := s.subscription.IsActive(
		ctx,
		ownerID,
	)

	// Pengecekan langganan gagal: kembalikan error.
	if err != nil {
		return err
	}

	// Langganan tidak aktif: kembalikan ErrSubscriptionInactive.
	if !isSubscribed {
		return apperror.ErrSubscriptionInactive
	}
	return nil
}

// validateOwnership mengambil antrian dan memastikan requester adalah owner-nya; selain itu ErrNotFound.
func (s *QueueService) validateOwnership(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, error) {
	// Ambil antrian dari repository berdasarkan ID-nya.
	queue, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	// Antrian tidak ditemukan atau gagal dibaca: kembalikan error.
	if err != nil {
		return model.Queue{}, err
	}
	// Requester bukan owner: sembunyikan keberadaan antrian dengan ErrNotFound.
	if requesterID != queue.OwnerID {
		return model.Queue{}, apperror.ErrNotFound
	}
	return queue, nil
}
