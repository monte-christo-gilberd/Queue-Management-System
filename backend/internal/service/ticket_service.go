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
		queueID int64,
	) (model.Ticket, error)
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

var errNoAccountFinder = errors.New("ticket service: account finder is not configured")

var errNoQueueGuard = errors.
	New("ticket service: queue guard is not configured")

type TicketService struct {
	repository TicketRepository
	tx         TxRunner
	notifier   EventNotifier
	guard      QueueGuard
	accounts   AccountFinder
}

func NewTicketService(
	repository TicketRepository,
	tx TxRunner,
	notifiers ...EventNotifier,
) *TicketService {
	svc := &TicketService{
		repository: repository,
		tx:         tx,
	}
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

const SortKeyGap int64 = 65536

const maxRetries int = 3

func (s *TicketService) JoinQueue(
	ctx context.Context,
	queueID, userID int64,
) (model.Ticket, error) {
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, err := s.guard.GetActiveQueue(
		ctx,
		queueID,
	); err != nil {
		return model.Ticket{}, err
	}

	var retry int = 0
	var sortKey int64
	var ticket model.Ticket

	for retry < maxRetries {
		// Tabel tickets membatasi 1 tiket per (queue, user), jadi tabrakan UNIQUE bisa berarti
		// "sudah join" (bukan tabrakan nomor). Dicek di tiap percobaan: kalau dua join user yang
		// sama berebut, percobaan ulang akan menemukan tiket yang sudah dibuat pemenangnya.
		joined, err := s.repository.HasTicket(
			ctx,
			queueID,
			userID,
		)
		if err != nil {
			return model.Ticket{}, err
		}
		// Join ulang tidak membuat nomor kedua: tiket yang sudah ada dikembalikan (spek 4.2, TC-05-04).
		if joined {
			return s.repository.FindByUser(
				ctx,
				queueID,
				userID,
			)
		}

		if err := s.tx.WithTx(
			ctx,
			func(tx *sqlx.Tx) error {
				var err error
				sortKey, err = s.repository.MaxWaitingSortKey(
					ctx,
					tx,
					queueID,
				)
				if err != nil {
					return err
				}

				ticket, err = s.repository.Insert(
					ctx,
					tx,
					queueID,
					userID,
					sortKey+SortKeyGap,
				)
				if err != nil {
					return err
				}

				return nil
			},
		); err != nil {
			if errors.Is(err, apperror.ErrDuplicate) {
				retry++
				continue
			} else {
				return model.Ticket{}, err
			}
		}
		break
	}

	if retry == maxRetries {
		return model.Ticket{}, apperror.ErrRetryExhausted
	}

	notifyEvent(
		s.notifier,
		queueID,
		EventTicketJoined,
		dto.NewTicketEvent(ticket),
	)

	return ticket, nil
}

func (s *TicketService) NextTicket(
	ctx context.Context,
	queueID,
	requesterID int64,
	counterID int64,
) (model.Ticket, error) {
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(
		ctx,
		queueID,
		requesterID,
	); err != nil {
		return model.Ticket{}, err
	}

	var counter model.Counter
	var ticket model.Ticket
	var err error

	// counter_id 0 = counter default antrian. Counter lain harus milik antrian ini (bila tidak, ErrNotFound).
	if counterID == 0 {
		counter, err = s.repository.GetCounterByQueue(ctx, queueID)
	} else {
		counter, err = s.repository.GetCounter(ctx, queueID, counterID)
	}
	if err != nil {
		return model.Ticket{}, err
	}
	counterID = counter.ID

	err = s.tx.WithTx(
		ctx,
		func(tx *sqlx.Tx) error {
			// Urutan kunci: nomor yang sedang dilayani dulu, baru waiting teratas. CancelTicket dan
			// RemoveTicket memakai urutan yang sama agar tidak terjadi deadlock.
			serving, servingErr := s.repository.CurrentServing(ctx, tx, queueID)
			if servingErr != nil && !errors.Is(servingErr, apperror.ErrNotFound) {
				return servingErr
			}
			var err error
			ticket, err = s.repository.NextWaiting(
				ctx,
				tx,
				queueID,
			)
			if err != nil {
				return err
			}

			// Nomor yang sedang dilayani selesai, lalu tiket berikutnya mulai dilayani.
			// Antrian tanpa nomor yang sedang dilayani tetap sah: langkah pertama dilewati.
			if servingErr == nil {
				if err := s.repository.MarkCompleted(ctx, tx, serving.ID); err != nil {
					return err
				}
			}

			return s.repository.MarkServing(
				ctx,
				tx,
				ticket.ID,
				counterID,
			)
		})

	if errors.Is(err, apperror.ErrNotFound) {
		return model.Ticket{}, apperror.ErrQueueEmpty
	}
	if err != nil {
		return model.Ticket{}, err
	}

	// Payload mengikuti keadaan setelah Next: nomor ini sekarang sedang dilayani (spek 4.3 langkah 3).
	ticket.Status = "currently_served"
	ticket.CalledAt = sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}
	ticket.CalledCounterID = counterID
	if !counter.IsDefault {
		ticket.CounterName = counter.Name
	}

	notifyEvent(
		s.notifier,
		queueID,
		EventTicketNext,
		dto.NewTicketEvent(ticket),
	)

	return ticket, nil
}

// SkipTicket melewati tiket yang sedang dilayani lalu memanggil tiket waiting berikutnya.
// Bila tidak ada yang dilayani: ErrNothingToSkip; bila tidak ada yang menunggu: ErrQueueEmpty
// (transaksi dibatalkan, tiket tetap dilayani). Mengembalikan tiket yang baru dipanggil.
func (s *TicketService) SkipTicket(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Ticket, error) {
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}

	counter, err := s.repository.GetCounterByQueue(ctx, queueID)
	if err != nil {
		return model.Ticket{}, err
	}

	var skipped, called model.Ticket
	var requeued bool
	var newSortKey int64
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		skipped, err = s.repository.CurrentServing(ctx, tx, queueID)
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrNothingToSkip
		}
		if err != nil {
			return err
		}
		called, err = s.repository.NextWaiting(ctx, tx, queueID)
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrQueueEmpty
		}
		if err != nil {
			return err
		}
		if err := s.repository.MarkServing(ctx, tx, called.ID, counter.ID); err != nil {
			return err
		}
		settings, err := s.repository.SkipSettings(ctx, tx, queueID)
		if err != nil {
			return err
		}
		// Nomor yang sudah dikembalikan sebanyak batas menjadi skipped permanen.
		if settings.Fallback == model.SkipFallbackRemove || skipped.SkipCount >= settings.MaxReturns {
			return s.repository.MarkSkipped(ctx, tx, skipped.ID)
		}
		sortKey, err := s.requeueSortKey(ctx, tx, queueID, settings)
		if err != nil {
			return err
		}
		requeued = true
		newSortKey = sortKey
		return s.repository.RequeueSkipped(ctx, tx, skipped.ID, sortKey)
	})
	if err != nil {
		return model.Ticket{}, err
	}

	skipped.SkipCount++
	skipped.CompletedAt = sql.NullTime{}
	if requeued {
		skipped.Status = "waiting"
		skipped.SortKey = newSortKey
	} else {
		skipped.Status = "skipped"
	}

	called.Status = "currently_served"
	called.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
	called.CalledCounterID = counter.ID
	if !counter.IsDefault {
		called.CounterName = counter.Name
	}

	notifyEvent(s.notifier, queueID, EventTicketSkipped, dto.NewTicketEvent(skipped))
	notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(called))

	return called, nil
}

// retireOutcome: hasil satu pengeluaran tiket dari antrian (cancel atau remove).
type retireOutcome struct {
	changed     bool         // false bila tiket sudah tidak aktif (no-op)
	target      model.Ticket // keadaan tiket yang dikeluarkan
	promoted    model.Ticket // waiting teratas yang naik ke currently_served, bila ada
	hasPromoted bool
}

// retireTicket mengeluarkan satu tiket aktif dari antrian dalam satu transaksi. Bila tiket itu sedang dilayani,
// waiting teratas naik ke currently_served. Tiket yang sudah tidak aktif menghasilkan no-op tanpa menulis apa pun.
// mark menulis status akhir (MarkCancelled atau MarkDeleted); status adalah nilai yang dipakai di payload.
func (s *TicketService) retireTicket(
	ctx context.Context,
	queueID,
	ticketID int64,
	mark func(ctx context.Context, tx *sqlx.Tx, ticketID int64) error,
	status string,
) (retireOutcome, error) {
	counter, err := s.repository.GetCounterByQueue(ctx, queueID)
	if err != nil {
		return retireOutcome{}, err
	}

	var out retireOutcome
	err = s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Urutan kunci sama dengan NextTicket: nomor yang sedang dilayani dulu, lalu tiket target.
		if _, err := s.repository.CurrentServing(ctx, tx, queueID); err != nil && !errors.Is(err, apperror.ErrNotFound) {
			return err
		}
		target, err := s.repository.GetForUpdate(ctx, tx, queueID, ticketID)
		if err != nil {
			return err
		}
		out.target = target
		// Status dibaca di dalam transaksi, jadi perubahan yang terjadi sejak pemanggil membaca tetap terdeteksi.
		if target.Status != "waiting" && target.Status != "currently_served" {
			return nil
		}

		if err := mark(ctx, tx, target.ID); err != nil {
			return err
		}
		out.changed = true
		out.target.Status = status
		out.target.CompletedAt = sql.NullTime{}

		if target.Status != "currently_served" {
			return nil
		}
		next, err := s.repository.NextWaiting(ctx, tx, queueID)
		if errors.Is(err, apperror.ErrNotFound) {
			return nil // tidak ada yang menunggu: antrian kosong, tidak ada yang dilayani
		}
		if err != nil {
			return err
		}
		if err := s.repository.MarkServing(ctx, tx, next.ID, counter.ID); err != nil {
			return err
		}
		next.Status = "currently_served"
		next.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
		next.CalledCounterID = counter.ID
		if !counter.IsDefault {
			next.CounterName = counter.Name
		}
		out.promoted = next
		out.hasPromoted = true
		return nil
	})
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
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, err := s.guard.GetActiveQueue(ctx, queueID); err != nil {
		return model.Ticket{}, err
	}

	mine, err := s.repository.FindByUser(ctx, queueID, userID)
	if err != nil {
		return model.Ticket{}, err
	}

	out, err := s.retireTicket(ctx, queueID, mine.ID, s.repository.MarkCancelled, "cancelled")
	if err != nil {
		return model.Ticket{}, err
	}
	if !out.changed {
		return out.target, nil
	}

	out.target.CancelledAt = sql.NullTime{Time: time.Now(), Valid: true}
	notifyEvent(s.notifier, queueID, EventTicketCancelled, dto.NewTicketEvent(out.target))
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
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	if err != nil {
		return model.Ticket{}, err
	}
	if queue.Status == "closed" {
		return model.Ticket{}, apperror.ErrQueueClosed
	}

	out, err := s.retireTicket(ctx, queueID, ticketID, s.repository.MarkDeleted, "deleted")
	if err != nil {
		return model.Ticket{}, err
	}
	if !out.changed {
		return out.target, nil
	}

	notifyEvent(s.notifier, queueID, EventTicketRemoved, dto.NewTicketEvent(out.target))
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
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if s.accounts == nil {
		return model.Ticket{}, errNoAccountFinder
	}
	if position < 1 {
		return model.Ticket{}, apperror.ErrInvalidPosition
	}
	queue, err := s.guard.GetQueue(ctx, queueID, requesterID)
	if err != nil {
		return model.Ticket{}, err
	}
	if queue.Status == "closed" {
		return model.Ticket{}, apperror.ErrQueueClosed
	}

	users, err := s.accounts.FindByLogin(ctx, identifier)
	if err != nil {
		return model.Ticket{}, err
	}
	switch {
	case len(users) == 0:
		return model.Ticket{}, apperror.ErrNotFound
	case len(users) > 1:
		return model.Ticket{}, apperror.ErrAmbiguousAccount
	}
	userID := users[0].ID

	joined, err := s.repository.HasTicket(ctx, queueID, userID)
	if err != nil {
		return model.Ticket{}, err
	}
	if joined {
		return model.Ticket{}, apperror.ErrAlreadyJoined
	}

	var ticket model.Ticket
	for retry := 0; retry < maxRetries; retry++ {
		err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
			sortKey, err := s.sortKeyAt(ctx, tx, queueID, position)
			if err != nil {
				return err
			}
			ticket, err = s.repository.Insert(ctx, tx, queueID, userID, sortKey)
			return err
		})
		if err == nil {
			notifyEvent(s.notifier, queueID, EventTicketJoined, dto.NewTicketEvent(ticket))
			return ticket, nil
		}
		if !errors.Is(err, apperror.ErrDuplicate) {
			return model.Ticket{}, err
		}
		// Bentrok sort_key: coba lagi. Bentrok UNIQUE (queue, user) berarti akun itu baru saja ikut antrian.
		joined, err := s.repository.HasTicket(ctx, queueID, userID)
		if err != nil {
			return model.Ticket{}, err
		}
		if joined {
			return model.Ticket{}, apperror.ErrAlreadyJoined
		}
	}
	return model.Ticket{}, apperror.ErrRetryExhausted
}

// RecallDelay: jeda minimal setelah sebuah nomor dipanggil sebelum owner boleh memanggil ulang.
const RecallDelay = 3 * time.Second

// RecallTicket memanggil ulang nomor yang sedang dilayani: hanya push ke pemiliknya (lewat trigger database),
// tidak mengubah status. Boleh sekali per pemanggilan dan baru 3 detik setelah dipanggil.
func (s *TicketService) RecallTicket(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Ticket, error) {
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}

	var serving model.Ticket
	err := s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		serving, err = s.repository.CurrentServing(ctx, tx, queueID)
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrNothingToRecall
		}
		if err != nil {
			return err
		}
		if serving.RecalledAt.Valid {
			return apperror.ErrAlreadyRecalled
		}
		if time.Since(serving.CalledAt.Time) < RecallDelay {
			return apperror.ErrRecallTooSoon
		}
		return s.repository.MarkRecalled(ctx, tx, serving.ID)
	})
	if err != nil {
		return model.Ticket{}, err
	}
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
	if settings.Fallback != model.SkipFallbackAfterN || settings.N < 1 {
		return s.backSortKey(ctx, tx, queueID)
	}
	return s.sortKeyAt(ctx, tx, queueID, settings.N+1)
}

// backSortKey: sort_key di paling belakang antrian.
func (s *TicketService) backSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (int64, error) {
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
	for attempt := 0; attempt < 2; attempt++ {
		keys, err := s.repository.WaitingSortKeys(ctx, tx, queueID, position)
		if err != nil {
			return 0, err
		}
		if len(keys) < position {
			return s.backSortKey(ctx, tx, queueID)
		}
		var lower int64
		if position >= 2 {
			lower = keys[position-2]
		}
		upper := keys[position-1]
		mid := lower + (upper-lower)/2
		if mid > lower {
			taken, err := s.repository.SortKeyTaken(ctx, tx, queueID, mid)
			if err != nil {
				return 0, err
			}
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
func (s *TicketService) ListTickets(
	ctx context.Context,
	queueID,
	requesterID int64,
	statuses []string,
) ([]model.Ticket, error) {
	if s.guard == nil {
		return nil, errNoQueueGuard
	}

	if _, err := s.guard.GetQueue(
		ctx,
		queueID,
		requesterID,
	); err != nil {
		return nil, err
	}

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
	if s.guard == nil {
		return model.QueueStatus{}, errNoQueueGuard
	}

	queue, _, err := s.guard.ViewQueue(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return model.QueueStatus{}, err
	}

	status := model.QueueStatus{Queue: queue}

	number, ok, err := s.repository.CurrentServedNumber(
		ctx,
		queueID,
	)
	if err != nil {
		return model.QueueStatus{}, err
	}
	if ok {
		status.CurrentNumber = &number
		// Nama counter default disembunyikan (is_default); bila counter non-default
		// hadir, nama-nya ikut tampil untuk customer & owner.
		if counter, cerr := s.repository.GetCounterByQueue(ctx, queueID); cerr == nil && counter.ID != 0 && !counter.IsDefault {
			counterName := counter.Name
			status.CurrentCounterName = &counterName
		}
	}

	// Sebelumnya (completed terbaru, karena nomor yang sedang dilayani tidak ada di daftar completed) dan
	// berikutnya (waiting pertama): hanya nomor.
	recent, err := s.repository.RecentCompletedNumbers(
		ctx,
		queueID,
		1,
	)
	if err != nil {
		return model.QueueStatus{}, err
	}
	if len(recent) == 1 {
		status.PreviousNumber = &recent[0]
	}

	next, hasNext, err := s.repository.FirstWaitingNumber(
		ctx,
		queueID,
	)
	if err != nil {
		return model.QueueStatus{}, err
	}
	if hasNext {
		status.NextNumber = &next
	}

	mine, err := s.repository.FindByUser(
		ctx,
		queueID,
		requesterID,
	)
	if errors.Is(err, apperror.ErrNotFound) {
		return status, nil
	}
	if err != nil {
		return model.QueueStatus{}, err
	}
	status.MyTicket = &mine

	if mine.Status == "waiting" {
		ahead, err := s.repository.CountWaitingBefore(
			ctx,
			queueID,
			mine.SortKey,
		)
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
	return s.repository.ListByUser(ctx, userID)
}
