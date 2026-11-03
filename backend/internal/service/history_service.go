package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
)

const (
	DefaultHistoryQueuesLimit  = 20
	MaxHistoryQueuesLimit      = 100
	DefaultHistoryTicketsLimit = 50
	MaxHistoryTicketsLimit     = 200
)

type HistoryRepository interface {
	ListClosedQueues(
		ctx context.Context,
		ownerID int64,
		after *model.HistoryQueueCursor,
		limit int,
	) ([]model.HistoryQueue, error)
	GetClosedQueue(
		ctx context.Context,
		ownerID,
		queueID int64,
	) (model.HistoryQueue, error)
	ListQueueTickets(
		ctx context.Context,
		queueID int64,
		afterNumber,
		limit int,
	) ([]model.HistoryTicket, error)
}

// HistoryQueuesPage: satu halaman daftar riwayat. NextCursor kosong = tidak ada halaman berikutnya.
type HistoryQueuesPage struct {
	Queues     []model.HistoryQueue
	NextCursor string
}

// HistoryQueueDetail: ringkasan antrian dan satu halaman nomornya.
type HistoryQueueDetail struct {
	Queue      model.HistoryQueue
	Tickets    []model.HistoryTicket
	NextCursor string
}

// HistoryService: riwayat antrian owner (4.15). Hanya antrian yang sudah ditutup yang tampil; antrian aktif
// tetap di dashboard dan daftar antrian. Data dibaca langsung dari tabel live (D1), dan kepemilikan dicek
// di query, jadi non-owner mendapat daftar kosong atau ErrNotFound.
type HistoryService struct {
	repository HistoryRepository
}

func NewHistoryService(repository HistoryRepository) *HistoryService {
	return &HistoryService{repository: repository}
}

// ListQueues mengembalikan halaman daftar antrian yang sudah ditutup. Kursor tidak valid: ErrInvalidCursor.
func (s *HistoryService) ListQueues(
	ctx context.Context,
	ownerID int64,
	cursor string,
	limit int,
) (HistoryQueuesPage, error) {
	limit = clampLimit(limit, DefaultHistoryQueuesLimit, MaxHistoryQueuesLimit)
	after, err := decodeQueueCursor(cursor)
	if err != nil {
		return HistoryQueuesPage{}, err
	}

	// Ambil satu baris lebih untuk tahu apakah masih ada halaman berikutnya.
	rows, err := s.repository.ListClosedQueues(ctx, ownerID, after, limit+1)
	if err != nil {
		return HistoryQueuesPage{}, err
	}
	page := HistoryQueuesPage{Queues: rows}
	if len(rows) > limit {
		page.Queues = rows[:limit]
		last := page.Queues[limit-1]
		page.NextCursor = encodeQueueCursor(last.ClosedAt, last.ID)
	}
	return page, nil
}

// GetQueue mengembalikan ringkasan dan satu halaman nomor antrian yang sudah ditutup. Bukan milik owner,
// masih aktif, atau tidak ada: ErrNotFound.
func (s *HistoryService) GetQueue(
	ctx context.Context,
	ownerID,
	queueID int64,
	cursor string,
	limit int,
) (HistoryQueueDetail, error) {
	limit = clampLimit(limit, DefaultHistoryTicketsLimit, MaxHistoryTicketsLimit)
	after, err := decodeTicketCursor(cursor)
	if err != nil {
		return HistoryQueueDetail{}, err
	}

	queue, err := s.repository.GetClosedQueue(ctx, ownerID, queueID)
	if err != nil {
		return HistoryQueueDetail{}, err
	}
	rows, err := s.repository.ListQueueTickets(ctx, queueID, after, limit+1)
	if err != nil {
		return HistoryQueueDetail{}, err
	}
	detail := HistoryQueueDetail{Queue: queue, Tickets: rows}
	if len(rows) > limit {
		detail.Tickets = rows[:limit]
		detail.NextCursor = strconv.Itoa(detail.Tickets[limit-1].TicketNumber)
	}
	return detail, nil
}

func clampLimit(limit, def, max int) int {
	switch {
	case limit <= 0:
		return def
	case limit > max:
		return max
	default:
		return limit
	}
}

// Kursor daftar antrian: "<closed_at dalam mikrodetik>.<id>". Mikrodetik karena itu presisi Postgres, jadi
// perbandingan keyset tidak meleset.
func encodeQueueCursor(closedAt time.Time, id int64) string {
	return fmt.Sprintf("%d.%d", closedAt.UnixMicro(), id)
}

func decodeQueueCursor(cursor string) (*model.HistoryQueueCursor, error) {
	if cursor == "" {
		return nil, nil
	}
	parts := strings.Split(cursor, ".")
	if len(parts) != 2 {
		return nil, apperror.ErrInvalidCursor
	}
	micro, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || id <= 0 {
		return nil, apperror.ErrInvalidCursor
	}
	return &model.HistoryQueueCursor{ClosedAt: time.UnixMicro(micro).UTC(), ID: id}, nil
}

// Kursor daftar nomor: nomor tiket terakhir yang sudah dikirim.
func decodeTicketCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0, apperror.ErrInvalidCursor
	}
	return n, nil
}
