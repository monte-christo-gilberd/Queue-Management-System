package dto

import (
	"qms/internal/model"
	"time"
)

type CreateQueueRequest struct {
	QueueName        string `json:"queue_name" binding:"required,no_html_tags"`
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
}

type UpdateQueueRequest struct {
	QueueName        string `json:"queue_name" binding:"no_html_tags"`
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
}

type UpdateAuthzRequest struct {
	QueueID           int64 `json:"queue_id"`
	CanManageQueue    bool  `json:"can_manage_queue"`
	CanManageCounters bool  `json:"can_manage_counters"`
	CanManageQr       bool  `json:"can_manage_qr"`
	CanViewQueue      bool  `json:"can_view_queue"`
	CanManageDisplay  bool  `json:"can_manage_display"`
	CanContinueQueue  bool  `json:"can_continue_queue"`
	CanNotifyCustomer bool  `json:"can_notify_customer"`
	CanManageTickets  bool  `json:"can_manage_tickets"`
	CanManageNotes    bool  `json:"can_manage_notes"`
}

// QueueResponse adalah bentuk JSON queue ke klien (snake_case, field kosong = null).
type QueueResponse struct {
	ID        int64      `json:"id"`
	OwnerID   int64      `json:"owner_id"`
	QueueName string     `json:"queue_name"`
	QueueDesc *string    `json:"queue_desc"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

func NewQueueResponse(q model.Queue) QueueResponse {
	var desc *string
	if q.QueueDesc.Valid {
		desc = &q.QueueDesc.String
	}
	return QueueResponse{
		ID:        q.ID,
		OwnerID:   q.OwnerID,
		QueueName: q.QueueName,
		QueueDesc: desc,
		Status:    q.Status,
		CreatedAt: q.CreatedAt,
		ClosedAt:  nullTimePtr(q.ClosedAt),
	}
}

type PublicQueueResponse struct {
	ID        int64      `json:"id"`
	QueueName string     `json:"queue_name"`
	QueueDesc *string    `json:"queue_desc"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

func NewPublicQueueResponse(q model.Queue) PublicQueueResponse {
	full := NewQueueResponse(q)
	return PublicQueueResponse{
		ID:        full.ID,
		QueueName: full.QueueName,
		QueueDesc: full.QueueDesc,
		Status:    full.Status,
		CreatedAt: full.CreatedAt,
		ClosedAt:  full.ClosedAt,
	}
}

func NewQueueResponses(queues []model.Queue) []QueueResponse {
	out := make([]QueueResponse, 0, len(queues))
	for _, q := range queues {
		out = append(out, NewQueueResponse(q))
	}
	return out
}

type QueueAuthzResponse struct {
	QueueID           int64 `json:"queue_id"`
	CanManageQueue    bool  `json:"can_manage_queue"`
	CanManageCounters bool  `json:"can_manage_counters"`
	CanManageQr       bool  `json:"can_manage_qr"`
	CanViewQueue      bool  `json:"can_view_queue"`
	CanManageDisplay  bool  `json:"can_manage_display"`
	CanContinueQueue  bool  `json:"can_continue_queue"`
	CanNotifyCustomer bool  `json:"can_notify_customer"`
	CanManageTickets  bool  `json:"can_manage_tickets"`
	CanManageNotes    bool  `json:"can_manage_notes"`
}

func NewQueueAuthzResponse(a model.QueueAuthz) QueueAuthzResponse {
	return QueueAuthzResponse{
		QueueID:           a.QueueID,
		CanManageQueue:    a.CanManageQueue,
		CanManageCounters: a.CanManageCounters,
		CanManageQr:       a.CanManageQr,
		CanViewQueue:      a.CanViewQueue,
		CanManageDisplay:  a.CanManageDisplay,
		CanContinueQueue:  a.CanContinueQueue,
		CanNotifyCustomer: a.CanNotifyCustomer,
		CanManageTickets:  a.CanManageTickets,
		CanManageNotes:    a.CanManageNotes,
	}
}
