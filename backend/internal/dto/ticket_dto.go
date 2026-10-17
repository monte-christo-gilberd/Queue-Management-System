package dto

import (
	"database/sql"
	"qms/internal/model"
	"time"
)

type TicketResponse struct {
	ID             int64      `json:"id"`
	QueueID        int64      `json:"queue_id"`
	TicketNumber   int        `json:"ticket_number"`
	SortKey        int64      `json:"sort_key"`
	UserID         int64      `json:"user_id"`
	Status         string     `json:"status"`
	SkipCount      int        `json:"skip_count"`
	LastNotifiedAt *time.Time `json:"last_notified_at"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CancelledAt    *time.Time `json:"cancelled_at"`
}

func NewTicketResponse(t model.Ticket) TicketResponse {
	return TicketResponse{
		ID:             t.ID,
		QueueID:        t.QueueID,
		TicketNumber:   t.TicketNumber,
		SortKey:        t.SortKey,
		UserID:         t.UserID,
		Status:         t.Status,
		SkipCount:      t.SkipCount,
		LastNotifiedAt: nullTimePtr(t.LastNotifiedAt),
		CreatedAt:      t.CreatedAt,
		CompletedAt:    nullTimePtr(t.CompletedAt),
		CancelledAt:    nullTimePtr(t.CancelledAt),
	}
}

// Tanpa user_id & sort_key: penerima event bisa customer lain di queue yang sama.
type TicketEvent struct {
	ID           int64      `json:"id"`
	QueueID      int64      `json:"queue_id"`
	TicketNumber int        `json:"ticket_number"`
	Status       string     `json:"status"`
	SkipCount    int        `json:"skip_count"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CancelledAt  *time.Time `json:"cancelled_at"`
}

func NewTicketEvent(t model.Ticket) TicketEvent {
	return TicketEvent{
		ID:           t.ID,
		QueueID:      t.QueueID,
		TicketNumber: t.TicketNumber,
		Status:       t.Status,
		SkipCount:    t.SkipCount,
		CreatedAt:    t.CreatedAt,
		CompletedAt:  nullTimePtr(t.CompletedAt),
		CancelledAt:  nullTimePtr(t.CancelledAt),
	}
}

func NewTicketResponses(tickets []model.Ticket) []TicketResponse {
	out := make([]TicketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, NewTicketResponse(t))
	}
	return out
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

type QueueStatusResponse struct {
	Queue         PublicQueueResponse `json:"queue"`
	MyTicket      *TicketResponse     `json:"my_ticket"`
	CurrentNumber *int                `json:"current_number"`
	WaitingAhead  *int                `json:"waiting_ahead"`
}

func NewQueueStatusResponse(s model.QueueStatus) QueueStatusResponse {
	var mine *TicketResponse
	if s.MyTicket != nil {
		t := NewTicketResponse(*s.MyTicket)
		mine = &t
	}
	return QueueStatusResponse{
		Queue:         NewPublicQueueResponse(s.Queue),
		MyTicket:      mine,
		CurrentNumber: s.CurrentNumber,
		WaitingAhead:  s.WaitingAhead,
	}
}

// UserTicketResponse: tiket saya + info antriannya.
type UserTicketResponse struct {
	TicketResponse
	QueueName   string `json:"queue_name"`
	QueueStatus string `json:"queue_status"`
}

func NewUserTicketResponses(tickets []model.UserTicket) []UserTicketResponse {
	out := make([]UserTicketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, UserTicketResponse{
			TicketResponse: NewTicketResponse(t.Ticket),
			QueueName:      t.QueueName,
			QueueStatus:    t.QueueStatus,
		})
	}
	return out
}
