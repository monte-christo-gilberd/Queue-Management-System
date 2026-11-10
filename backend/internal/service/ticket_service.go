package service

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

// TicketRepository: penyimpanan tiket dan pengurutan antrian. Metode yang menerima tx dipakai di dalam
// transaksi. Pengurutan memakai sort_key (bilangan bulat, lihat SortKeyGap). Dipenuhi repository (ticket_repo.go).
type TicketRepository interface {
	MaxWaitingSortKey(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (int64, error)
	Insert(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID, userID, sortKey int64,
	) (model.Ticket, error)
	NextWaiting(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (model.Ticket, error)
	MarkCompleted(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	MarkServing(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
		counterID int64,
	) error
	MarkCancelled(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	MarkDeleted(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	GetForUpdate(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		ticketID int64,
	) (model.Ticket, error)
	CurrentServing(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) (model.Ticket, error)
	LockCounter(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) error
	ServingByQueue(
		ctx context.Context,
		queueID int64,
	) ([]model.Ticket, error)
	MarkSkipped(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	MarkRecalled(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	SkipSettings(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (model.SkipSettings, error)
	WaitingSortKeys(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
		limit int,
	) ([]int64, error)
	SortKeyTaken(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		sortKey int64,
	) (bool, error)
	RequeueSkipped(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID,
		sortKey int64,
	) error
	RebalanceWaiting(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		gap int64,
	) error
	ListByQueue(
		ctx context.Context,
		queueID int64,
		statuses []string,
	) ([]model.Ticket, error)
	HasTicket(
		ctx context.Context,
		queueID,
		userID int64,
	) (bool, error)
	FindByUser(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	CurrentServedNumber(
		ctx context.Context,
		queueID int64,
	) (number int, ok bool, err error)
	RecentCompletedNumbers(
		ctx context.Context,
		queueID int64,
		n int,
	) ([]int, error)
	FirstWaitingNumber(
		ctx context.Context,
		queueID int64,
	) (number int, ok bool, err error)
	GetCounterByQueue(
		ctx context.Context,
		queueID int64,
	) (model.Counter, error)
	GetCounter(
		ctx context.Context,
		queueID,
		counterID int64,
	) (model.Counter, error)
	ServingOfCounter(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		counterID int64,
	) (model.Ticket, error)
	ReturnToWaiting(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID,
		sortKey int64,
	) error
	CountWaitingBefore(
		ctx context.Context,
		queueID,
		sortKey int64,
	) (int, error)
	ListByUser(
		ctx context.Context,
		userID int64,
	) ([]model.UserTicket, error)
}

// AccountFinder mencari akun dari email atau username. Dipenuhi UserRepository.
// Username tidak unik, jadi hasilnya bisa lebih dari satu baris; email dicocokkan lebih dulu.
type AccountFinder interface {
	FindByLogin(
		ctx context.Context,
		identifier string,
	) ([]model.User, error)
}

// QueueGuard dipenuhi *QueueService.
//   - GetQueue: requester harus owner; selain itu apperror.ErrNotFound.
//   - GetActiveQueue: queue harus ada (ErrNotFound) dan aktif (ErrQueueClosed).
//   - ViewQueue: owner atau pemegang tiket; selain itu apperror.ErrNotFound.
type QueueGuard interface {
	GetQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, error)
	GetActiveQueue(
		ctx context.Context,
		queueID int64,
	) (model.Queue, error)
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, bool, error)
}

// Galat konfigurasi: dikembalikan bila dependency yang wajib untuk operasi itu belum dipasang (fail closed).
var errNoAccountFinder = errors.New("ticket service: account finder is not configured")

// errNoTicketReleaser: hapus counter butuh TicketService yang melepas nomornya (WithTicketReleaser di CounterService).
var errNoTicketReleaser = errors.New("counter service: ticket releaser is not configured")

// errNoQueueGuard: operasi tiket butuh QueueGuard (dipasang lewat WithQueueGuard).
var errNoQueueGuard = errors.
	New("ticket service: queue guard is not configured")

// TicketService: tiket customer dan operasi counter. Mencakup join, next, skip, recall, cancel, remove,
// sisip posisi (owner), dan status antrian. Aturan waktu dan urutan ada di sini; repository hanya menulis.
type TicketService struct {
	// repository: penyimpanan tiket dan pengurutan antrian (TicketRepository).
	repository TicketRepository
	// tx: menjalankan perubahan tiket dalam satu transaksi.
	tx TxRunner
	// notifier: mengirim event tiket (joined, next, skipped, dll.). Boleh nil.
	notifier EventNotifier
	// guard: memastikan queue ada dan requester berhak mengaksesnya. Nil = fail closed.
	guard QueueGuard
	// accounts: mencari akun untuk sisip posisi oleh owner (US-10). Nil = fail closed.
	accounts AccountFinder
	// verification: bila terpasang, JoinQueue menolak user yang emailnya belum diverifikasi.
	verification VerificationGuard
}

// NewTicketService membuat TicketService dengan repository dan TxRunner. Notifier opsional (variadic).
// Guard, pencarian akun, dan verifikasi dipasang sesudahnya lewat With...
func NewTicketService(
	repository TicketRepository,
	tx TxRunner,
	notifiers ...EventNotifier,
) *TicketService {
	svc := &TicketService{
		repository: repository,
		tx:         tx,
	}
	// Notifier hanya dipasang bila memang diberikan; tanpa notifier, event tidak dikirim.
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithQueueGuard memasang pengecekan queue. Tanpa guard, JoinQueue/NextTicket/ListTickets gagal (fail closed).
func (s *TicketService) WithQueueGuard(
	guard QueueGuard,
) *TicketService {
	s.guard = guard
	return s
}

// WithAccountFinder memasang pencarian akun untuk sisip (US-10). Tanpa ini, InsertTicket gagal tertutup.
func (s *TicketService) WithAccountFinder(
	accounts AccountFinder,
) *TicketService {
	s.accounts = accounts
	return s
}

// WithVerificationGuard mewajibkan email terverifikasi untuk mendapat nomor baru lewat JoinQueue
// (apperror.ErrEmailNotVerified). Tanpa guard (verifikasi email mati), tidak ada pemeriksaan.
func (s *TicketService) WithVerificationGuard(
	guard VerificationGuard,
) *TicketService {
	s.verification = guard
	return s
}

// SortKeyGap: jarak antar sort_key. Tiket baru di belakang mendapat maksimal + SortKeyGap, dan sisipan di antara
// dua tiket memakai titik tengahnya. Jarak yang habis ditangani dengan RebalanceWaiting.
const SortKeyGap int64 = 65536

// maxRetries: jumlah percobaan maksimal bila terjadi tabrakan (sort_key atau UNIQUE). Setelah itu ErrRetryExhausted.
const maxRetries int = 3

// JoinQueue memberi user nomor baru di belakang antrian. Join ulang mengembalikan tiket yang sudah ada (tidak
// membuat nomor kedua). Antrian harus aktif. Email harus terverifikasi bila verifikasi dinyalakan. Tabrakan
// diulang sampai maxRetries kali. Event ticket_joined dikirim setelah tiket dibuat.
func (s *TicketService) JoinQueue(
	ctx context.Context,
	queueID, userID int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan antrian tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Antrian harus ada dan aktif; bila tidak, galatnya langsung dikembalikan ke pemanggil.
	if _, err := s.guard.GetActiveQueue(
		ctx,
		queueID,
	); err != nil {
		return model.Ticket{}, err
	}

	var retry int = 0
	var sortKey int64
	var ticket model.Ticket

	// Ulangi pembuatan nomor selama belum melewati batas percobaan (maxRetries) akibat tabrakan.
	for retry < maxRetries {
		// Tabel tickets membatasi 1 tiket per (queue, user), jadi tabrakan UNIQUE bisa berarti
		// "sudah join" (bukan tabrakan nomor). Dicek di tiap percobaan: kalau dua join user yang
		// sama berebut, percobaan ulang akan menemukan tiket yang sudah dibuat pemenangnya.
		// Cek apakah user ini sudah punya tiket di antrian ini.
		joined, err := s.repository.HasTicket(
			ctx,
			queueID,
			userID,
		)
		// Gagal mengecek: kembalikan error ke pemanggil.
		if err != nil {
			return model.Ticket{}, err
		}
		// Join ulang tidak membuat nomor kedua: tiket yang sudah ada dikembalikan (spek 4.2, TC-05-04).
		if joined {
			// Ambil tiket milik user ini di antrian tersebut untuk dikembalikan.
			return s.repository.FindByUser(
				ctx,
				queueID,
				userID,
			)
		}

		// Hanya untuk nomor baru: user yang sudah punya tiket di atas tetap dikembalikan tiketnya walau
		// emailnya (belum) terverifikasi, jadi scan ulang tidak berubah menjadi galat.
		if s.verification != nil {
			// Minta verifikasi email user; bila belum terverifikasi, tolak dengan ErrEmailNotVerified.
			if err := s.verification.RequireVerified(ctx, userID); err != nil {
				return model.Ticket{}, err
			}
		}

		// Buat nomor baru di dalam satu transaksi; bila galat duplikat, percobaan diulang di loop.
		if err := s.tx.WithTx(
			ctx,
			func(tx *sqlx.Tx) error {
				var err error
				// Ambil sort_key tertinggi di antrian waiting sebagai dasar nomor baru di belakang.
				sortKey, err = s.repository.MaxWaitingSortKey(
					ctx,
					tx,
					queueID,
				)
				// Gagal membaca sort_key: batalkan transaksi dan kembalikan error.
				if err != nil {
					return err
				}

				// Simpan tiket baru dengan sort_key di belakang (maksimal + SortKeyGap).
				ticket, err = s.repository.Insert(
					ctx,
					tx,
					queueID,
					userID,
					sortKey+SortKeyGap,
				)
				// Gagal menyimpan: batalkan transaksi dan kembalikan error.
				if err != nil {
					return err
				}

				return nil
			},
		); err != nil {
			// Galat duplikat (tabrakan UNIQUE atau sort_key): tambah hitungan dan coba lagi.
			if errors.Is(err, apperror.ErrDuplicate) {
				retry++
				continue
			} else {
				// Galat lain bukan tabrakan: langsung kembalikan apa adanya.
				return model.Ticket{}, err
			}
		}
		break
	}

	// Semua percobaan habis karena tabrakan: kembalikan ErrRetryExhausted.
	if retry == maxRetries {
		return model.Ticket{}, apperror.ErrRetryExhausted
	}

	// Kirim event ticket_joined ke notifier; payload dibuat dari tiket baru lewat dto.NewTicketEvent.
	notifyEvent(
		s.notifier,
		queueID,
		EventTicketJoined,
		dto.NewTicketEvent(ticket),
	)

	return ticket, nil
}

// resolveCounter: counter yang dipilih. counter_id 0 = counter default antrian. Counter lain harus milik antrian
// ini, bila tidak ErrNotFound.
func (s *TicketService) resolveCounter(
	ctx context.Context,
	queueID,
	counterID int64,
) (model.Counter, error) {
	// counter_id 0 berarti counter default antrian.
	if counterID == 0 {
		// Ambil counter default milik antrian ini.
		return s.repository.GetCounterByQueue(ctx, queueID)
	}
	// Ambil counter yang dipilih dan pastikan counter itu milik antrian ini.
	return s.repository.GetCounter(ctx, queueID, counterID)
}

// checkExpected: pengaman aksi ganda (D9). expected nil = klien lama, tanpa pengecekan. Bila tidak nil, harus sama
// dengan nomor yang sedang dilayani counter ini (0 bila counter kosong), jika tidak 409 tanpa perubahan data.
func checkExpected(expected *int64, serving model.Ticket, servingFound bool) error {
	// Klien lama tidak mengirim nomor yang diharapkan: lewati pengecekan.
	if expected == nil {
		return nil
	}
	var current int64
	// Counter punya nomor yang sedang dilayani: pakai ID-nya sebagai nilai pembanding.
	if servingFound {
		current = serving.ID
	}
	// Nomor yang diharapkan klien berbeda dengan keadaan sekarang: tolak tanpa mengubah data.
	if *expected != current {
		return apperror.ErrCounterStateChanged
	}
	return nil
}

// NextTicket: counter yang dipilih menyelesaikan nomornya lalu menerima waiting teratas (4.11).
func (s *TicketService) NextTicket(
	ctx context.Context,
	queueID,
	requesterID,
	counterID int64,
	expected *int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; bila tidak, galat dikembalikan.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}
	// Tentukan counter yang dipilih (0 = counter default).
	counter, err := s.resolveCounter(ctx, queueID, counterID)
	// Gagal menentukan counter: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}

	var ticket model.Ticket
	// Seluruh langkah Next dijalankan dalam satu transaksi agar tidak setengah jalan.
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Urutan kunci: counter (FOR SHARE), nomor yang dilayani counter itu, lalu waiting teratas.
		if err := s.repository.LockCounter(ctx, tx, queueID, counter.ID); err != nil {
			return err
		}
		// Ambil nomor yang sedang dilayani counter ini; boleh tidak ada (counter sedang kosong).
		serving, servingErr := s.repository.CurrentServing(ctx, tx, queueID, counter.ID)
		// Galat selain "tidak ada nomor" dikembalikan; ErrNotFound berarti counter kosong dan proses lanjut.
		if servingErr != nil && !errors.Is(servingErr, apperror.ErrNotFound) {
			return servingErr
		}
		servingFound := servingErr == nil
		// Pengaman aksi ganda: tolak bila nomor yang dilayani sudah berubah dari yang diharapkan klien.
		if err := checkExpected(expected, serving, servingFound); err != nil {
			return err
		}

		// Ambil tiket waiting teratas di antrian sebagai nomor berikutnya.
		next, err := s.repository.NextWaiting(ctx, tx, queueID)
		// Tidak ada yang menunggu: kembalikan ErrQueueEmpty.
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrQueueEmpty
		}
		// Galat lain saat membaca antrian: kembalikan error.
		if err != nil {
			return err
		}
		ticket = next

		// Nomor yang dilayani counter ini selesai, lalu tiket berikutnya mulai dilayani di counter ini.
		// Bila counter sedang melayani nomor, tandai nomor itu selesai (completed).
		if servingFound {
			// Tandai nomor yang sedang dilayani sebagai completed.
			if err := s.repository.MarkCompleted(ctx, tx, serving.ID); err != nil {
				return err
			}
		}
		// Tandai tiket berikutnya sedang dilayani di counter ini, dalam transaksi yang sama.
		return s.repository.MarkServing(ctx, tx, ticket.ID, counter.ID)
	})
	// Transaksi gagal: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}

	// Payload mengikuti keadaan setelah Next: nomor ini sekarang sedang dilayani (spek 4.3 langkah 3).
	ticket.Status = "currently_served"
	// Catat waktu panggilan dengan waktu sekarang.
	ticket.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
	ticket.CalledCounterID = counter.ID
	// Counter bukan default: sertakan namanya di payload.
	if !counter.IsDefault {
		ticket.CounterName = counter.Name
	}

	// Kirim event ticket_next ke notifier dengan payload tiket yang baru dipanggil.
	notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(ticket))
	return ticket, nil
}

// SkipTicket melewati nomor yang sedang dilayani counter ini, lalu memanggil waiting teratas ke counter yang sama.
// Bila counter itu tidak melayani: ErrNothingToSkip; bila tidak ada yang menunggu: ErrQueueEmpty (tidak ada
// perubahan). Mengembalikan tiket yang baru dipanggil.
func (s *TicketService) SkipTicket(
	ctx context.Context,
	queueID,
	requesterID,
	counterID int64,
	expected *int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; bila tidak, galat dikembalikan.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}
	// Tentukan counter yang dipilih (0 = counter default).
	counter, err := s.resolveCounter(ctx, queueID, counterID)
	// Gagal menentukan counter: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}

	var skipped, called model.Ticket
	var requeued bool
	var newSortKey int64
	// Seluruh langkah skip dijalankan dalam satu transaksi agar tidak setengah jalan.
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Kunci counter agar tidak ada perubahan bersamaan selama transaksi.
		if err := s.repository.LockCounter(ctx, tx, queueID, counter.ID); err != nil {
			return err
		}
		// Ambil nomor yang sedang dilayani counter ini untuk dilewati.
		serving, err := s.repository.CurrentServing(ctx, tx, queueID, counter.ID)
		// Counter tidak sedang melayani: tidak ada yang bisa dilewati, kembalikan ErrNothingToSkip.
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrNothingToSkip
		}
		// Galat lain saat membaca nomor yang dilayani: kembalikan error.
		if err != nil {
			return err
		}
		// Pengaman aksi ganda: tolak bila nomor yang dilayani sudah berubah dari yang diharapkan klien.
		if err := checkExpected(expected, serving, true); err != nil {
			return err
		}
		skipped = serving

		// Ambil tiket waiting teratas yang akan dipanggil ke counter ini.
		called, err = s.repository.NextWaiting(ctx, tx, queueID)
		// Tidak ada yang menunggu: tidak ada perubahan, kembalikan ErrQueueEmpty.
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrQueueEmpty
		}
		// Galat lain saat membaca antrian: kembalikan error.
		if err != nil {
			return err
		}
		// Nomor yang dilewati turun dari "sedang dilayani" lebih dulu, supaya counter yang sama tidak memegang
		// dua nomor sekaligus (indeks tickets_one_serving_per_counter). Urutannya sama dengan NextTicket.
		if err := s.repository.MarkCompleted(ctx, tx, skipped.ID); err != nil {
			return err
		}
		// Tandai tiket waiting teratas sedang dilayani di counter ini.
		if err := s.repository.MarkServing(ctx, tx, called.ID, counter.ID); err != nil {
			return err
		}
		// Ambil aturan skip antrian (fallback dan batas pengembalian).
		settings, err := s.repository.SkipSettings(ctx, tx, queueID)
		// Gagal membaca aturan skip: kembalikan error.
		if err != nil {
			return err
		}
		// Nomor yang sudah dikembalikan sebanyak batas menjadi skipped permanen.
		if settings.Fallback == model.SkipFallbackRemove || skipped.SkipCount >= settings.MaxReturns {
			// Tandai skipped permanen: nomor tidak dikembalikan ke waiting.
			return s.repository.MarkSkipped(ctx, tx, skipped.ID)
		}
		// Hitung sort_key untuk mengembalikan nomor yang dilewati ke antrian waiting.
		sortKey, err := s.requeueSortKey(ctx, tx, queueID, settings)
		// Gagal menghitung sort_key: kembalikan error.
		if err != nil {
			return err
		}
		requeued = true
		newSortKey = sortKey
		// Kembalikan nomor yang dilewati ke waiting dengan sort_key yang sudah dihitung.
		return s.repository.RequeueSkipped(ctx, tx, skipped.ID, sortKey)
	})
	// Transaksi gagal: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}

	skipped.SkipCount++
	skipped.CompletedAt = sql.NullTime{}
	// Nomor berhasil dikembalikan ke waiting: status waiting dan sort_key yang baru.
	if requeued {
		skipped.Status = "waiting"
		skipped.SortKey = newSortKey
	} else {
		skipped.Status = "skipped"
	}

	called.Status = "currently_served"
	// Catat waktu panggilan dengan waktu sekarang.
	called.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
	called.CalledCounterID = counter.ID
	// Counter bukan default: sertakan namanya di payload.
	if !counter.IsDefault {
		called.CounterName = counter.Name
	}

	// Kirim event ticket_skipped untuk nomor yang dilewati.
	notifyEvent(s.notifier, queueID, EventTicketSkipped, dto.NewTicketEvent(skipped))
	// Kirim event ticket_next untuk nomor yang baru dipanggil.
	notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(called))

	return called, nil
}

// retireOutcome: hasil satu pengeluaran tiket dari antrian (cancel atau remove).
type retireOutcome struct {
	changed  bool         // false bila tiket sudah tidak aktif (no-op)
	target   model.Ticket // keadaan tiket yang dikeluarkan
	promoted model.Ticket // waiting teratas yang naik ke currently_served, bila ada
	// hasPromoted: true bila promoted berisi tiket yang naik ke currently_served.
	hasPromoted bool
}

// retireTicket mengeluarkan satu tiket aktif dari antrian dalam satu transaksi. Bila tiket itu sedang dilayani
// counter C, waiting teratas naik ke counter C yang sama. Tiket tidak aktif: no-op tanpa menulis apa pun.
// mark menulis status akhir (MarkCancelled atau MarkDeleted); status adalah nilai yang dipakai di payload.
func (s *TicketService) retireTicket(
	ctx context.Context,
	queueID,
	ticketID int64,
	mark func(ctx context.Context, tx *sqlx.Tx, ticketID int64) error,
	status string,
) (retireOutcome, error) {
	var out retireOutcome
	// Seluruh pengeluaran tiket dijalankan dalam satu transaksi.
	err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Tiket target dikunci dulu (bila sedang dilayani, baris itu adalah nomor counter-nya).
		// Ambil tiket target dengan kunci baris agar tidak diubah pihak lain selama transaksi.
		target, err := s.repository.GetForUpdate(ctx, tx, queueID, ticketID)
		// Gagal membaca tiket: kembalikan error.
		if err != nil {
			return err
		}
		out.target = target
		// Status dibaca di dalam transaksi, jadi perubahan yang terjadi sejak pemanggil membaca tetap terdeteksi.
		// Tiket sudah tidak aktif: no-op, tidak ada yang ditulis.
		if target.Status != "waiting" && target.Status != "currently_served" {
			return nil
		}
		// Tulis status akhir (cancelled atau deleted) lewat fungsi mark yang diberikan pemanggil.
		if err := mark(ctx, tx, target.ID); err != nil {
			return err
		}
		out.changed = true
		out.target.Status = status
		out.target.CompletedAt = sql.NullTime{}

		// Hanya tiket yang sedang dilayani yang memegang counter; tiket waiting cukup selesai di sini.
		if target.Status != "currently_served" {
			return nil
		}
		// Counter yang melepas nomor ini menerima waiting teratas.
		counterID := target.CalledCounterID
		// Ambil counter yang melepas nomor ini (untuk tahu apakah counter-nya default).
		counter, err := s.repository.GetCounter(ctx, queueID, counterID)
		// Gagal membaca counter: kembalikan error.
		if err != nil {
			return err
		}
		// Ambil tiket waiting teratas sebagai pengganti.
		next, err := s.repository.NextWaiting(ctx, tx, queueID)
		// Tidak ada yang menunggu: counter dibiarkan kosong, tidak ada yang naik.
		if errors.Is(err, apperror.ErrNotFound) {
			return nil // tidak ada yang menunggu: counter ini kosong
		}
		// Galat lain saat membaca antrian: kembalikan error.
		if err != nil {
			return err
		}
		// Waiting teratas naik ke counter yang sama.
		if err := s.repository.MarkServing(ctx, tx, next.ID, counterID); err != nil {
			return err
		}
		next.Status = "currently_served"
		// Catat waktu panggilan dengan waktu sekarang.
		next.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
		next.CalledCounterID = counterID
		// Counter bukan default: sertakan namanya di payload.
		if !counter.IsDefault {
			next.CounterName = counter.Name
		}
		out.promoted = next
		out.hasPromoted = true
		return nil
	})
	// Transaksi gagal: kembalikan error ke pemanggil.
	if err != nil {
		return retireOutcome{}, err
	}
	return out, nil
}

// CancelTicket membatalkan tiket aktif milik customer (4.9). Nomor yang sedang dilayani dibatalkan, lalu waiting
// teratas naik. Tanpa tiket aktif, atau sudah tidak aktif: no-op tanpa error (klik dua kali tidak error).
func (s *TicketService) CancelTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan antrian tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Antrian harus ada dan aktif; bila tidak, galatnya langsung dikembalikan.
	if _, err := s.guard.GetActiveQueue(ctx, queueID); err != nil {
		return model.Ticket{}, err
	}

	// Cari tiket milik user di antrian ini.
	mine, err := s.repository.FindByUser(ctx, queueID, userID)
	// Gagal mencari tiket: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}

	// Keluarkan tiket dengan status "cancelled"; waiting teratas naik bila tiket sedang dilayani.
	out, err := s.retireTicket(ctx, queueID, mine.ID, s.repository.MarkCancelled, "cancelled")
	// Gagal mengeluarkan tiket: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Tidak ada perubahan (tiket sudah tidak aktif): kembalikan keadaan tiket apa adanya tanpa event.
	if !out.changed {
		return out.target, nil
	}

	// Catat waktu pembatalan dengan waktu sekarang.
	out.target.CancelledAt = sql.NullTime{Time: time.Now(), Valid: true}
	// Kirim event ticket_cancelled untuk tiket yang dibatalkan.
	notifyEvent(s.notifier, queueID, EventTicketCancelled, dto.NewTicketEvent(out.target))
	// Ada waiting yang naik ke counter: kirim event ticket_next untuk tiket itu.
	if out.hasPromoted {
		notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(out.promoted))
	}
	return out.target, nil
}

// RemoveTicket menghapus nomor customer dari antrian atas perintah owner (4.10). Perilakunya seperti skip tanpa
// fallback: tiket tidak dikembalikan ke waiting dan skip_count tidak naik. Antrian ditutup: ErrQueueClosed.
// Tiket sudah tidak aktif: no-op. Tiket milik antrian lain: ErrNotFound.
func (s *TicketService) RemoveTicket(
	ctx context.Context,
	queueID,
	requesterID,
	ticketID int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; ambil data antrian untuk pengecekan status.
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	// Gagal membaca antrian atau bukan owner: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Antrian sudah ditutup: tidak boleh menghapus nomor, kembalikan ErrQueueClosed.
	if queue.Status == "closed" {
		return model.Ticket{}, apperror.ErrQueueClosed
	}

	// Keluarkan tiket dengan status "deleted"; waiting teratas naik bila tiket sedang dilayani.
	out, err := s.retireTicket(ctx, queueID, ticketID, s.repository.MarkDeleted, "deleted")
	// Gagal mengeluarkan tiket: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Tidak ada perubahan (tiket sudah tidak aktif): kembalikan keadaan tiket apa adanya tanpa event.
	if !out.changed {
		return out.target, nil
	}

	// Kirim event ticket_removed untuk tiket yang dihapus.
	notifyEvent(s.notifier, queueID, EventTicketRemoved, dto.NewTicketEvent(out.target))
	// Ada waiting yang naik ke counter: kirim event ticket_next untuk tiket itu.
	if out.hasPromoted {
		notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(out.promoted))
	}
	return out.target, nil
}

// InsertTicket menyisipkan akun terdaftar ke posisi tertentu di antrian (US-10 / 4.12). Position 1-based,
// dihitung dari tiket waiting: 1 = berikutnya. Posisi melebihi panjang antrian masuk di paling belakang.
// Akun dicari lewat identifier (email atau username); username ganda ditolak agar akun yang benar tidak tertukar.
func (s *TicketService) InsertTicket(
	ctx context.Context,
	queueID,
	requesterID int64,
	identifier string,
	position int,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Tanpa AccountFinder, akun tidak bisa dicari: tolak (fail closed).
	if s.accounts == nil {
		return model.Ticket{}, errNoAccountFinder
	}
	// Posisi dimulai dari 1; nilai kurang dari 1 tidak valid.
	if position < 1 {
		return model.Ticket{}, apperror.ErrInvalidPosition
	}
	// Pastikan requester adalah owner antrian; ambil data antrian untuk pengecekan status.
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	// Gagal membaca antrian atau bukan owner: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Antrian sudah ditutup: tidak boleh menyisipkan akun, kembalikan ErrQueueClosed.
	if queue.Status == "closed" {
		return model.Ticket{}, apperror.ErrQueueClosed
	}

	// Cari akun berdasarkan email atau username.
	users, err := s.accounts.FindByLogin(ctx, identifier)
	// Gagal mencari akun: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Periksa jumlah akun yang cocok: 0 berarti tidak ada, lebih dari 1 berarti ambigu.
	switch {
	// Tidak ada akun yang cocok: kembalikan ErrNotFound.
	case len(users) == 0:
		return model.Ticket{}, apperror.ErrNotFound
	// Lebih dari satu akun cocok (username ganda): tolak agar tidak salah sisip.
	case len(users) > 1:
		return model.Ticket{}, apperror.ErrAmbiguousAccount
	}
	userID := users[0].ID

	// Cek apakah akun itu sudah punya tiket di antrian ini.
	joined, err := s.repository.HasTicket(ctx, queueID, userID)
	// Gagal mengecek: kembalikan error.
	if err != nil {
		return model.Ticket{}, err
	}
	// Akun sudah ikut antrian: tolak dengan ErrAlreadyJoined.
	if joined {
		return model.Ticket{}, apperror.ErrAlreadyJoined
	}

	var ticket model.Ticket
	// Coba menyisipkan tiket sampai maxRetries kali bila terjadi tabrakan.
	for retry := 0; retry < maxRetries; retry++ {
		// Sisipkan tiket di dalam satu transaksi, dengan sort_key sesuai posisi yang diminta.
		err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
			// Hitung sort_key yang membuat tiket menempati posisi yang diminta.
			sortKey, err := s.sortKeyAt(ctx, tx, queueID, position)
			// Gagal menghitung sort_key: batalkan transaksi dan kembalikan error.
			if err != nil {
				return err
			}
			// Simpan tiket baru dengan sort_key tersebut.
			ticket, err = s.repository.Insert(ctx, tx, queueID, userID, sortKey)
			return err
		})
		// Penyisipan berhasil: kirim event ticket_joined dan kembalikan tiket baru.
		if err == nil {
			notifyEvent(s.notifier, queueID, EventTicketJoined, dto.NewTicketEvent(ticket))
			return ticket, nil
		}
		// Galat bukan duplikat: kembalikan error apa adanya.
		if !errors.Is(err, apperror.ErrDuplicate) {
			return model.Ticket{}, err
		}
		// Bentrok sort_key: coba lagi. Bentrok UNIQUE (queue, user) berarti akun itu baru saja ikut antrian.
		// Cek ulang apakah akun itu sudah punya tiket di antrian ini.
		joined, err := s.repository.HasTicket(ctx, queueID, userID)
		// Gagal mengecek: kembalikan error.
		if err != nil {
			return model.Ticket{}, err
		}
		// Ternyata akun sudah ikut antrian: tolak dengan ErrAlreadyJoined.
		if joined {
			return model.Ticket{}, apperror.ErrAlreadyJoined
		}
	}
	return model.Ticket{}, apperror.ErrRetryExhausted
}

// RecallDelay: jeda minimal setelah sebuah nomor dipanggil sebelum owner boleh memanggil ulang.
const RecallDelay = 3 * time.Second

// RecallTicket memanggil ulang nomor yang sedang dilayani counter ini: hanya push ke pemiliknya (lewat trigger
// database), tidak mengubah status. Boleh sekali per pemanggilan dan baru 3 detik setelah dipanggil.
func (s *TicketService) RecallTicket(
	ctx context.Context,
	queueID,
	requesterID,
	counterID int64,
	expected *int64,
) (model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; bila tidak, galat dikembalikan.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}
	// Tentukan counter yang dipilih (0 = counter default).
	counter, err := s.resolveCounter(ctx, queueID, counterID)
	// Gagal menentukan counter: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}

	var serving model.Ticket
	// Seluruh pengecekan dan penandaan recall dijalankan dalam satu transaksi.
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Kunci counter agar tidak ada perubahan bersamaan selama transaksi.
		if err := s.repository.LockCounter(ctx, tx, queueID, counter.ID); err != nil {
			return err
		}
		// Ambil nomor yang sedang dilayani counter ini untuk dipanggil ulang.
		serving, err = s.repository.CurrentServing(ctx, tx, queueID, counter.ID)
		// Counter tidak sedang melayani: tidak ada yang bisa dipanggil ulang, kembalikan ErrNothingToRecall.
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrNothingToRecall
		}
		// Galat lain saat membaca nomor yang dilayani: kembalikan error.
		if err != nil {
			return err
		}
		// Pengaman aksi ganda: tolak bila nomor yang dilayani sudah berubah dari yang diharapkan klien.
		if err := checkExpected(expected, serving, true); err != nil {
			return err
		}
		// Nomor ini sudah pernah dipanggil ulang: tolak dengan ErrAlreadyRecalled.
		if serving.RecalledAt.Valid {
			return apperror.ErrAlreadyRecalled
		}
		// Belum melewati jeda minimal sejak dipanggil (dihitung dengan time.Since): tolak dengan ErrRecallTooSoon.
		if time.Since(serving.CalledAt.Time) < RecallDelay {
			return apperror.ErrRecallTooSoon
		}
		// Tandai nomor sebagai sudah dipanggil ulang (hanya mencatat waktu, status tidak berubah).
		return s.repository.MarkRecalled(ctx, tx, serving.ID)
	})
	// Transaksi gagal: kembalikan error ke pemanggil.
	if err != nil {
		return model.Ticket{}, err
	}
	// Catat waktu pemanggilan ulang dengan waktu sekarang.
	serving.RecalledAt = sql.NullTime{Time: time.Now(), Valid: true}
	return serving, nil
}

// requeueSortKey: sort_key untuk tiket yang dikembalikan. Dipanggil setelah tiket berikutnya diambil,
// jadi "mundur N" dihitung terhadap antrian yang tersisa: tiket masuk tepat di posisi N+1.
// Bila yang menunggu kurang dari N, jatuh ke belakang.
func (s *TicketService) requeueSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	settings model.SkipSettings,
) (int64, error) {
	// Aturan fallback bukan "mundur N" (atau N tidak valid): tiket masuk di paling belakang.
	if settings.Fallback != model.SkipFallbackAfterN || settings.N < 1 {
		// Hitung sort_key untuk paling belakang antrian.
		return s.backSortKey(ctx, tx, queueID)
	}
	// Hitung sort_key untuk posisi N+1 (mundur N dari depan antrian).
	return s.sortKeyAt(ctx, tx, queueID, settings.N+1)
}

// ReleaseCounterTicket melepas nomor yang sedang dilayani counter ini sebelum counter dihapus (4.11, D2): nomor
// kembali ke posisi 1 urutan tunggu, skip_count tidak naik. Counter tanpa nomor yang dilayani: tidak ada yang
// dilakukan. Dipanggil dalam transaksi penghapusan counter.
func (s *TicketService) ReleaseCounterTicket(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) error {
	// Ambil nomor yang sedang dilayani counter ini; boleh tidak ada.
	ticket, err := s.repository.ServingOfCounter(ctx, tx, queueID, counterID)
	// Counter tanpa nomor yang dilayani: tidak ada yang perlu dilepas, selesai tanpa error.
	if errors.Is(err, apperror.ErrNotFound) {
		return nil
	}
	// Galat lain saat membaca nomor yang dilayani: kembalikan error.
	if err != nil {
		return err
	}
	// Hitung sort_key untuk posisi 1 (paling depan) agar nomor kembali didahulukan.
	sortKey, err := s.sortKeyAt(ctx, tx, queueID, 1)
	// Gagal menghitung sort_key: kembalikan error.
	if err != nil {
		return err
	}
	// Kembalikan nomor ke waiting dengan sort_key posisi 1.
	return s.repository.ReturnToWaiting(ctx, tx, ticket.ID, sortKey)
}

// backSortKey: sort_key di paling belakang antrian.
// backSortKey: sort_key yang membuat tiket masuk di paling belakang, yaitu maksimal waiting + SortKeyGap.
func (s *TicketService) backSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (int64, error) {
	// Ambil sort_key tertinggi di antrian waiting sebagai dasar.
	max, err := s.repository.MaxWaitingSortKey(ctx, tx, queueID)
	return max + SortKeyGap, err
}

// sortKeyAt: sort_key yang membuat tiket menempati posisi (1-based, dihitung dari tiket waiting) di antrian.
// Posisi 1 berarti paling depan. Bila tidak ada tiket di posisi itu (antrian lebih pendek), tiket masuk di
// paling belakang. Sisipan di antara dua tiket memakai titik tengah sort_key; bila jaraknya habis atau
// bentrok, sort_key tiket waiting dinomori ulang lalu dihitung lagi.
func (s *TicketService) sortKeyAt(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	position int,
) (int64, error) {
	// Maksimal dua kali percobaan: bila jarak habis atau bentrok, nomori ulang lalu hitung lagi.
	for attempt := 0; attempt < 2; attempt++ {
		// Ambil sort_key tiket waiting sampai posisi yang diminta.
		keys, err := s.repository.WaitingSortKeys(ctx, tx, queueID, position)
		// Gagal membaca sort_key: kembalikan error.
		if err != nil {
			return 0, err
		}
		// Antrian lebih pendek dari posisi yang diminta: tiket masuk di paling belakang.
		if len(keys) < position {
			return s.backSortKey(ctx, tx, queueID)
		}
		var lower int64
		// Posisi 2 ke atas punya tiket di depannya: ambil sort_key tiket itu sebagai batas bawah.
		if position >= 2 {
			lower = keys[position-2]
		}
		upper := keys[position-1]
		mid := lower + (upper-lower)/2
		// Masih ada ruang di antara dua sort_key: coba titik tengahnya.
		if mid > lower {
			// Cek apakah titik tengah itu sudah dipakai tiket lain.
			taken, err := s.repository.SortKeyTaken(ctx, tx, queueID, mid)
			// Gagal mengecek: kembalikan error.
			if err != nil {
				return 0, err
			}
			// Titik tengah masih kosong: pakai sebagai sort_key.
			if !taken {
				return mid, nil
			}
		}
		// Jarak habis atau bentrok: nomori ulang tiket waiting lalu hitung lagi.
		if err := s.repository.RebalanceWaiting(ctx, tx, queueID, SortKeyGap); err != nil {
			return 0, err
		}
	}
	return 0, apperror.ErrRetryExhausted
}

// ListTickets mengembalikan tiket queue milik requester. statuses kosong = semua status.
// ListTickets mengembalikan tiket queue milik requester. statuses kosong = semua status.
func (s *TicketService) ListTickets(
	ctx context.Context,
	queueID,
	requesterID int64,
	statuses []string,
) ([]model.Ticket, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return nil, errNoQueueGuard
	}

	// Pastikan requester adalah owner antrian; bila tidak, galat dikembalikan.
	if _, err := s.guard.GetQueue(
		ctx,
		queueID,
		requesterID,
	); err != nil {
		return nil, err
	}

	// Ambil tiket antrian yang sesuai filter status (kosong = semua status).
	return s.repository.ListByQueue(
		ctx,
		queueID,
		statuses,
	)
}

// QueueStatus mengembalikan nomor yang sedang dilayani + tiket requester untuk satu queue.
// Boleh dipanggil owner atau pemegang tiket; selain itu apperror.ErrNotFound.
func (s *TicketService) QueueStatus(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.QueueStatus, error) {
	// Tanpa QueueGuard, pengecekan hak akses tidak bisa dilakukan: tolak (fail closed).
	if s.guard == nil {
		return model.QueueStatus{}, errNoQueueGuard
	}

	// Pastikan requester adalah owner atau pemegang tiket antrian; ambil data antrian untuk status.
	queue, _, err := s.guard.ViewQueue(
		ctx,
		queueID,
		requesterID,
	)
	// Gagal membaca antrian atau tidak berhak melihat: kembalikan error.
	if err != nil {
		return model.QueueStatus{}, err
	}

	status := model.QueueStatus{Queue: queue}

	// Semua counter yang sedang melayani. Nomor yang paling baru dipanggil menjadi current_number (kompatibel
	// dengan klien lama). Nama counter default disembunyikan (is_default).
	// Ambil daftar tiket yang sedang dilayani di setiap counter antrian ini.
	servings, err := s.repository.ServingByQueue(ctx, queueID)
	// Gagal membaca daftar nomor yang dilayani: kembalikan error.
	if err != nil {
		return model.QueueStatus{}, err
	}
	// Isi tiap entri dengan nomor yang dilayani dan, bila counter bukan default, namanya.
	for _, t := range servings {
		entry := model.ServingEntry{CounterID: t.CalledCounterID, TicketNumber: t.TicketNumber}
		// Ambil counter tersebut; bila berhasil dan bukan default, sertakan namanya.
		if counter, cerr := s.repository.GetCounter(ctx, queueID, t.CalledCounterID); cerr == nil && !counter.IsDefault {
			name := counter.Name
			entry.CounterName = &name
		}
		status.Serving = append(status.Serving, entry)
	}
	// Ada counter yang melayani: nomor pertama (paling baru dipanggil) menjadi current_number.
	if len(status.Serving) > 0 {
		number := status.Serving[0].TicketNumber
		status.CurrentNumber = &number
		status.CurrentCounterName = status.Serving[0].CounterName
	}

	// Sebelumnya (completed terbaru, karena nomor yang sedang dilayani tidak ada di daftar completed) dan
	// berikutnya (waiting pertama): hanya nomor.
	// Ambil 1 nomor completed terbaru sebagai nomor sebelumnya.
	recent, err := s.repository.RecentCompletedNumbers(
		ctx,
		queueID,
		1,
	)
	// Gagal membaca nomor sebelumnya: kembalikan error.
	if err != nil {
		return model.QueueStatus{}, err
	}
	// Ada nomor completed terbaru: isi sebagai nomor sebelumnya.
	if len(recent) == 1 {
		status.PreviousNumber = &recent[0]
	}

	// Ambil nomor waiting pertama sebagai nomor berikutnya; hasNext menandai apakah ada.
	next, hasNext, err := s.repository.FirstWaitingNumber(
		ctx,
		queueID,
	)
	// Gagal membaca nomor berikutnya: kembalikan error.
	if err != nil {
		return model.QueueStatus{}, err
	}
	// Ada nomor waiting: isi sebagai nomor berikutnya.
	if hasNext {
		status.NextNumber = &next
	}

	// Cari tiket milik requester di antrian ini.
	mine, err := s.repository.FindByUser(
		ctx,
		queueID,
		requesterID,
	)
	// User tidak punya tiket di antrian ini: kembalikan status tanpa my_ticket.
	if errors.Is(err, apperror.ErrNotFound) {
		return status, nil
	}
	// Galat lain saat mencari tiket: kembalikan error.
	if err != nil {
		return model.QueueStatus{}, err
	}
	status.MyTicket = &mine

	// Hanya tiket yang masih menunggu yang dihitung posisinya di depan user.
	if mine.Status == "waiting" {
		// Hitung berapa tiket waiting di depan tiket user (berdasarkan sort_key).
		ahead, err := s.repository.CountWaitingBefore(
			ctx,
			queueID,
			mine.SortKey,
		)
		// Gagal menghitung: kembalikan error.
		if err != nil {
			return model.QueueStatus{}, err
		}
		status.WaitingAhead = &ahead
	}

	return status, nil
}

// ListMyTickets mengembalikan tiket milik user di semua antrian (tanpa pengecekan owner: hanya milik user sendiri).
func (s *TicketService) ListMyTickets(
	ctx context.Context,
	userID int64,
) ([]model.UserTicket, error) {
	// Ambil semua tiket milik user ini dari setiap antrian.
	return s.repository.ListByUser(ctx, userID)
}
