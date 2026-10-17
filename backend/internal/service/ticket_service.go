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
	) (model.Queue, model.QueueAuthz, error)
	GetActiveQueue(
		ctx context.Context,
		queueID int64,
	) (model.Queue, error)
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, model.QueueAuthz, bool, error)
}

var errNoQueueGuard = errors.New("ticket service: queue guard is not configured")

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
func (s *TicketService) WithQueueGuard(guard QueueGuard) *TicketService {
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
	if _, err := s.guard.GetActiveQueue(ctx, queueID); err != nil {
		return model.Ticket{}, err
	}

	var retry int = 0
	var sortKey int64
	var ticket model.Ticket

	for retry < maxRetries {
		// Tabel tickets membatasi 1 tiket per (queue, user), jadi tabrakan UNIQUE bisa berarti
		// "sudah join" (bukan tabrakan nomor). Dicek di tiap percobaan: kalau dua join user yang
		// sama berebut, percobaan ulang akan menemukan tiket yang sudah dibuat pemenangnya.
		joined, err := s.repository.HasTicket(ctx, queueID, userID)
		if err != nil {
			return model.Ticket{}, err
		}
		// Join ulang tidak membuat nomor kedua: tiket yang sudah ada dikembalikan (spek 4.2, TC-05-04).
		if joined {
			return s.repository.FindByUser(ctx, queueID, userID)
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

	notifyEvent(s.notifier, queueID, EventTicketJoined, dto.NewTicketEvent(ticket))

	return ticket, nil
}

func (s *TicketService) NextTicket(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Ticket, error) {
	if s.guard == nil {
		return model.Ticket{}, errNoQueueGuard
	}
	if _, _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.Ticket{}, err
	}

	var ticket model.Ticket

	err := s.tx.WithTx(
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
	ticket.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	notifyEvent(s.notifier, queueID, EventTicketNext, dto.NewTicketEvent(ticket))

	return ticket, nil
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

	if _, _, err := s.guard.GetQueue(
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

	queue, _, _, err := s.guard.ViewQueue(
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
	}

	// Sebelumnya (completed sebelum yang sedang dilayani) dan berikutnya (waiting pertama): hanya nomor.
	recent, err := s.repository.RecentCompletedNumbers(ctx, queueID, 2)
	if err != nil {
		return model.QueueStatus{}, err
	}
	if len(recent) == 2 {
		status.PreviousNumber = &recent[1]
	}

	next, hasNext, err := s.repository.FirstWaitingNumber(ctx, queueID)
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
