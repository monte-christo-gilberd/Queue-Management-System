package dto

import (
	"time"

	"qms/internal/model"
)

// HistoryQueueResponse: ringkasan antrian yang sudah ditutup. Hitungan per hasil akhir menjumlah ke Total.
type HistoryQueueResponse struct {
	ID           int64     `json:"id"`
	QueueName    string    `json:"queue_name"`
	TicketPrefix string    `json:"ticket_prefix"`
	OpenedAt     time.Time `json:"opened_at"`
	ClosedAt     time.Time `json:"closed_at"`
	Total        int       `json:"total"`
	Completed    int       `json:"completed"`
	Skipped      int       `json:"skipped"`
	Cancelled    int       `json:"cancelled"`
	Deleted      int       `json:"deleted"`
	Unserved     int       `json:"unserved"`
}

func NewHistoryQueueResponse(q model.HistoryQueue) HistoryQueueResponse {
	return HistoryQueueResponse{
		ID:           q.ID,
		QueueName:    q.QueueName,
		TicketPrefix: q.TicketPrefix,
		OpenedAt:     q.OpenedAt,
		ClosedAt:     q.ClosedAt,
		Total:        q.Total,
		Completed:    q.Completed,
		Skipped:      q.Skipped,
		Cancelled:    q.Cancelled,
		Deleted:      q.Deleted,
		Unserved:     q.Unserved,
	}
}

// HistoryTicketResponse: satu nomor di antrian yang sudah ditutup. Dibaca owner saja, jadi memuat email
// customer dan catatan owner.
type HistoryTicketResponse struct {
	TicketID      int64      `json:"ticket_id"`
	TicketNumber  int        `json:"ticket_number"`
	Outcome       string     `json:"outcome"`
	SkipCount     int        `json:"skip_count"`
	JoinedAt      time.Time  `json:"joined_at"`
	CalledAt      *time.Time `json:"called_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CancelledAt   *time.Time `json:"cancelled_at"`
	CustomerName  string     `json:"customer_name"`
	CustomerEmail string     `json:"customer_email"`
	CounterName   *string    `json:"counter_name"`
	Note          *string    `json:"note"`
}

func NewHistoryTicketResponse(t model.HistoryTicket) HistoryTicketResponse {
	out := HistoryTicketResponse{
		TicketID:      t.TicketID,
		TicketNumber:  t.TicketNumber,
		Outcome:       model.HistoryOutcome(t.Status),
		SkipCount:     t.SkipCount,
		JoinedAt:      t.JoinedAt,
		CustomerName:  t.CustomerName,
		CustomerEmail: t.CustomerEmail,
	}
	if t.CalledAt.Valid {
		out.CalledAt = &t.CalledAt.Time
	}
	if t.CompletedAt.Valid {
		out.CompletedAt = &t.CompletedAt.Time
	}
	if t.CancelledAt.Valid {
		out.CancelledAt = &t.CancelledAt.Time
	}
	if t.CounterName.Valid {
		out.CounterName = &t.CounterName.String
	}
	if t.Note.Valid {
		out.Note = &t.Note.String
	}
	return out
}

// HistoryQueuesResponse: GET /history/queues. NextCursor null = halaman terakhir.
type HistoryQueuesResponse struct {
	Action     string                 `json:"action"`
	Queues     []HistoryQueueResponse `json:"queues"`
	NextCursor *string                `json:"next_cursor"`
}

func NewHistoryQueuesResponse(queues []model.HistoryQueue, nextCursor string) HistoryQueuesResponse {
	out := HistoryQueuesResponse{
		Action: "list_history_queues",
		Queues: make([]HistoryQueueResponse, 0, len(queues)),
	}
	for _, q := range queues {
		out.Queues = append(out.Queues, NewHistoryQueueResponse(q))
	}
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}

// HistoryQueueDetailResponse: GET /history/queues/:queue_id.
type HistoryQueueDetailResponse struct {
	Action     string                  `json:"action"`
	Queue      HistoryQueueResponse    `json:"queue"`
	Tickets    []HistoryTicketResponse `json:"tickets"`
	NextCursor *string                 `json:"next_cursor"`
}

func NewHistoryQueueDetailResponse(queue model.HistoryQueue, tickets []model.HistoryTicket, nextCursor string) HistoryQueueDetailResponse {
	out := HistoryQueueDetailResponse{
		Action:  "history_queue",
		Queue:   NewHistoryQueueResponse(queue),
		Tickets: make([]HistoryTicketResponse, 0, len(tickets)),
	}
	for _, t := range tickets {
		out.Tickets = append(out.Tickets, NewHistoryTicketResponse(t))
	}
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}

// UserHistoryTicketResponse: satu tiket di riwayat customer. Sengaja tanpa catatan owner dan tanpa data user lain.
type UserHistoryTicketResponse struct {
	TicketID     int64      `json:"ticket_id"`
	QueueID      int64      `json:"queue_id"`
	QueueName    string     `json:"queue_name"`
	TicketPrefix string     `json:"ticket_prefix"`
	TicketNumber int        `json:"ticket_number"`
	Outcome      string     `json:"outcome"`
	QueueClosed  bool       `json:"queue_closed"`
	SkipCount    int        `json:"skip_count"`
	JoinedAt     time.Time  `json:"joined_at"`
	CalledAt     *time.Time `json:"called_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CancelledAt  *time.Time `json:"cancelled_at"`
	CounterName  *string    `json:"counter_name"`
}

func NewUserHistoryTicketResponse(t model.UserHistoryTicket) UserHistoryTicketResponse {
	out := UserHistoryTicketResponse{
		TicketID:     t.TicketID,
		QueueID:      t.QueueID,
		QueueName:    t.QueueName,
		TicketPrefix: t.TicketPrefix,
		TicketNumber: t.TicketNumber,
		Outcome:      model.HistoryOutcome(t.Status),
		QueueClosed:  t.QueueStatus == "closed",
		SkipCount:    t.SkipCount,
		JoinedAt:     t.JoinedAt,
	}
	if t.CalledAt.Valid {
		out.CalledAt = &t.CalledAt.Time
	}
	if t.CompletedAt.Valid {
		out.CompletedAt = &t.CompletedAt.Time
	}
	if t.CancelledAt.Valid {
		out.CancelledAt = &t.CancelledAt.Time
	}
	if t.CounterName.Valid {
		out.CounterName = &t.CounterName.String
	}
	return out
}

// MyHistoryResponse: GET /users/me/history. NextCursor null = halaman terakhir.
type MyHistoryResponse struct {
	Action     string                      `json:"action"`
	Tickets    []UserHistoryTicketResponse `json:"tickets"`
	NextCursor *string                     `json:"next_cursor"`
}

func NewMyHistoryResponse(tickets []model.UserHistoryTicket, nextCursor string) MyHistoryResponse {
	out := MyHistoryResponse{
		Action:  "list_my_history",
		Tickets: make([]UserHistoryTicketResponse, 0, len(tickets)),
	}
	for _, t := range tickets {
		out.Tickets = append(out.Tickets, NewUserHistoryTicketResponse(t))
	}
	if nextCursor != "" {
		out.NextCursor = &nextCursor
	}
	return out
}
