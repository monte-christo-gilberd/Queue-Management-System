package dto

import (
	"database/sql"
	"qms/internal/model"
	"time"
)

type TicketResponse struct {
	ID                int64      `json:"id"`
	QueueID           int64      `json:"queue_id"`
	TicketNumber      int        `json:"ticket_number"`
	SortKey           int64      `json:"sort_key"`
	UserID            int64      `json:"user_id"`
	Status            string     `json:"status"`
	SkipCount         int        `json:"skip_count"`
	LastNotifiedAt    *time.Time `json:"last_notified_at"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	CancelledAt       *time.Time `json:"cancelled_at"`
	CalledCounterID   *int64     `json:"called_counter_id"`
	CalledCounterName *string    `json:"called_counter_name"`
	CalledAt          *time.Time `json:"called_at"`
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
		CalledCounterID: func() *int64 {
			if t.CalledCounterID == 0 {
				return nil
			}
			v := t.CalledCounterID
			return &v
		}(),
		CalledAt: nullTimePtr(t.CalledAt),
	}
}

// Tanpa user_id & sort_key: penerima event bisa customer lain di queue yang sama.
type TicketEvent struct {
	ID                int64      `json:"id"`
	QueueID           int64      `json:"queue_id"`
	TicketNumber      int        `json:"ticket_number"`
	Status            string     `json:"status"`
	SkipCount         int        `json:"skip_count"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	CancelledAt       *time.Time `json:"cancelled_at"`
	CalledCounterID   *int64     `json:"called_counter_id"`
	CalledCounterName *string    `json:"called_counter_name"`
	CalledAt          *time.Time `json:"called_at"`
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
		CalledCounterID: func() *int64 {
			if t.CalledCounterID == 0 {
				return nil
			}
			v := t.CalledCounterID
			return &v
		}(),
		CalledCounterName: counterNamePtr(t.CounterName),
		CalledAt:          nullTimePtr(t.CalledAt),
	}
}

// OwnerTicketResponse: tiket untuk daftar tiket milik owner. Menambah catatan owner di atas TicketResponse.
// Hanya endpoint owner yang boleh memakainya; respons untuk customer tetap TicketResponse tanpa catatan.
type OwnerTicketResponse struct {
	TicketResponse
	Note          *string    `json:"note"`
	NoteUpdatedAt *time.Time `json:"note_updated_at"`
}

func NewOwnerTicketResponse(t model.Ticket) OwnerTicketResponse {
	out := OwnerTicketResponse{TicketResponse: NewTicketResponse(t)}
	if t.Note.Valid {
		out.Note = &t.Note.String
	}
	if t.NoteUpdatedAt.Valid {
		out.NoteUpdatedAt = &t.NoteUpdatedAt.Time
	}
	return out
}

func NewOwnerTicketResponses(tickets []model.Ticket) []OwnerTicketResponse {
	out := make([]OwnerTicketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, NewOwnerTicketResponse(t))
	}
	return out
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

func counterNamePtr(name string) *string {
	if name == "" {
		return nil
	}
	return &name
}

type QueueStatusResponse struct {
	Queue              PublicQueueResponse    `json:"queue"`
	MyTicket           *TicketResponse        `json:"my_ticket"`
	CurrentNumber      *int                   `json:"current_number"`
	CurrentCounterName *string                `json:"current_counter_name"`
	WaitingAhead       *int                   `json:"waiting_ahead"`
	PreviousNumber     *int                   `json:"previous_number"`
	NextNumber         *int                   `json:"next_number"`
	Serving            []ServingEntryResponse `json:"serving"`
}

// ServingEntryResponse: satu counter yang sedang melayani nomor (status antrian).
type ServingEntryResponse struct {
	CounterID    int64   `json:"counter_id"`
	CounterName  *string `json:"counter_name"`
	TicketNumber int     `json:"ticket_number"`
}

func NewQueueStatusResponse(s model.QueueStatus) QueueStatusResponse {
	var mine *TicketResponse
	if s.MyTicket != nil {
		t := NewTicketResponse(*s.MyTicket)
		mine = &t
	}
	var currentCounterName *string
	if s.CurrentCounterName != nil {
		currentCounterName = s.CurrentCounterName
	}
	return QueueStatusResponse{
		Queue:              NewPublicQueueResponse(s.Queue),
		MyTicket:           mine,
		CurrentNumber:      s.CurrentNumber,
		CurrentCounterName: currentCounterName,
		WaitingAhead:       s.WaitingAhead,
		PreviousNumber:     s.PreviousNumber,
		NextNumber:         s.NextNumber,
		Serving:            servingResponses(s.Serving),
	}
}

func servingResponses(entries []model.ServingEntry) []ServingEntryResponse {
	out := make([]ServingEntryResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, ServingEntryResponse{CounterID: e.CounterID, CounterName: e.CounterName, TicketNumber: e.TicketNumber})
	}
	return out
}

// UserTicketResponse: tiket saya + info antriannya.
type UserTicketResponse struct {
	TicketResponse
	QueueName     string `json:"queue_name"`
	TicketPrefix  string `json:"ticket_prefix"`
	QueueStatus   string `json:"queue_status"`
	CurrentNumber *int   `json:"current_number"`
}

func NewUserTicketResponses(tickets []model.UserTicket) []UserTicketResponse {
	out := make([]UserTicketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, UserTicketResponse{
			TicketResponse: NewTicketResponse(t.Ticket),
			QueueName:      t.QueueName,
			TicketPrefix:   t.TicketPrefix,
			QueueStatus:    t.QueueStatus,
			CurrentNumber:  t.CurrentNumber,
		})
	}
	return out
}
