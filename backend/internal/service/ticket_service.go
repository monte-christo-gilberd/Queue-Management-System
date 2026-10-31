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
		counterID int64,
	) error
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
	LastCompletedNumber(
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

var errNoQueueGuard = errors.
	New("ticket service: queue guard is not configured")

type TicketService struct {
	repository TicketRepository
	tx         TxRunner
	notifier   EventNotifier
	guard      QueueGuard
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

	counter, err = s.repository.GetCounterByQueue(ctx, queueID)
	if err != nil {
		return model.Ticket{}, err
	}
	if counterID == 0 {
		counterID = counter.ID
	}

	err = s.tx.WithTx(
		ctx,
		func(tx *sqlx.Tx) error {
			var err error
			ticket, err = s.repository.NextWaiting(
				ctx,
				tx,
				queueID,
			)
			if err != nil {
				return err
			}
			return s.repository.MarkCompleted(
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

	// Payload mengikuti keadaan setelah Next: nomor ini sudah selesai dilayani (spek 4.3 langkah 3).
	ticket.Status = "completed"
	ticket.CompletedAt = sql.NullTime{
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
		if err := s.repository.MarkCompleted(ctx, tx, called.ID, counter.ID); err != nil {
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

	called.Status = "completed"
	called.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	called.CalledAt = called.CompletedAt
	called.CalledCounterID = counter.ID
	if !counter.IsDefault {
		called.CounterName = counter.Name
	}

	notifyEvent(s.notifier, queueID, EventTicketSkipped, dto.NewTicketEvent(skipped))
	notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(called))

	return called, nil
}

// requeueSortKey: sort_key untuk tiket yang dikembalikan. Dipanggil setelah tiket berikutnya diambil,
// jadi "mundur N" dihitung terhadap antrian yang tersisa. Bila yang menunggu kurang dari N, jatuh ke belakang.
func (s *TicketService) requeueSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	settings model.SkipSettings,
) (int64, error) {
	back := func() (int64, error) {
		max, err := s.repository.MaxWaitingSortKey(ctx, tx, queueID)
		return max + SortKeyGap, err
	}
	if settings.Fallback != model.SkipFallbackAfterN || settings.N < 1 {
		return back()
	}

	for attempt := 0; attempt < 2; attempt++ {
		keys, err := s.repository.WaitingSortKeys(ctx, tx, queueID, settings.N+1)
		if err != nil {
			return 0, err
		}
		if len(keys) <= settings.N {
			return back()
		}
		lower, upper := keys[settings.N-1], keys[settings.N]
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

	number, ok, err := s.repository.LastCompletedNumber(
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

	// Sebelumnya (completed sebelum yang sedang dilayani) dan berikutnya (waiting pertama): hanya nomor.
	recent, err := s.repository.RecentCompletedNumbers(
		ctx,
		queueID,
		2,
	)
	if err != nil {
		return model.QueueStatus{}, err
	}
	if len(recent) == 2 {
		status.PreviousNumber = &recent[1]
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
