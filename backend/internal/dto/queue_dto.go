package dto

import (
	"qms/internal/model"
	"strings"
	"time"
)

// DefaultTicketPrefix dipakai bila owner tidak mengisi prefix.
const DefaultTicketPrefix = "A"

// NormalizeTicketPrefix: huruf besar, 1-3 karakter A-Z/0-9. Kosong dikembalikan apa adanya (ok=true).
func NormalizeTicketPrefix(raw string) (string, bool) {
	p := strings.ToUpper(strings.TrimSpace(raw))
	if p == "" {
		return "", true
	}
	if len(p) > 3 {
		return "", false
	}
	for _, r := range p {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", false
		}
	}
	return p, true
}

type CreateQueueRequest struct {
	QueueName        string `json:"queue_name" binding:"required,no_html_tags"`
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
	// Opsional: 1-3 huruf/angka. Kosong = "A" (create) / tidak diubah (update).
	TicketPrefix string `json:"ticket_prefix"`
}

type UpdateQueueRequest struct {
	QueueName        string `json:"queue_name" binding:"required,no_html_tags"`
	QueueDescription string `json:"queue_description" binding:"no_html_tags"`
	// Opsional: 1-3 huruf/angka. Kosong = "A" (create) / tidak diubah (update).
	TicketPrefix string `json:"ticket_prefix"`
}

type QueueResponse struct {
	ID           int64      `json:"id"`
	OwnerID      int64      `json:"owner_id"`
	QueueName    string     `json:"queue_name"`
	QueueDesc    *string    `json:"queue_desc"`
	TicketPrefix string     `json:"ticket_prefix"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ClosedAt     *time.Time `json:"closed_at"`
}

func NewQueueResponse(q model.Queue) QueueResponse {
	var desc *string
	if q.QueueDesc.Valid {
		desc = &q.QueueDesc.String
	}
	return QueueResponse{
		ID:           q.ID,
		OwnerID:      q.OwnerID,
		QueueName:    q.QueueName,
		QueueDesc:    desc,
		TicketPrefix: q.TicketPrefix,
		Status:       q.Status,
		CreatedAt:    q.CreatedAt,
		ClosedAt:     nullTimePtr(q.ClosedAt),
	}
}

type PublicQueueResponse struct {
	ID           int64      `json:"id"`
	QueueName    string     `json:"queue_name"`
	QueueDesc    *string    `json:"queue_desc"`
	TicketPrefix string     `json:"ticket_prefix"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ClosedAt     *time.Time `json:"closed_at"`
}

func NewPublicQueueResponse(q model.Queue) PublicQueueResponse {
	full := NewQueueResponse(q)
	return PublicQueueResponse{
		ID:           full.ID,
		QueueName:    full.QueueName,
		QueueDesc:    full.QueueDesc,
		TicketPrefix: full.TicketPrefix,
		Status:       full.Status,
		CreatedAt:    full.CreatedAt,
		ClosedAt:     full.ClosedAt,
	}
}

func NewQueueResponses(queues []model.Queue) []QueueResponse {
	out := make([]QueueResponse, 0, len(queues))
	for _, q := range queues {
		out = append(out, NewQueueResponse(q))
	}
	return out
}

// QueueQRResponse: respons endpoint QR owner.
// `action` memakai nama yang sama pola action lain ("queue_qr").
type QueueQRResponse struct {
	Action    string `json:"action"`
	QueueID   int64  `json:"queue_id"`
	Type      string `json:"type"`
	JoinURL   string `json:"join_url"`
	ExpiresAt *int64 `json:"expires_at"`
}

func NewQueueQRResponse(
	action string,
	queueID int64,
	qrType string,
	joinURL string,
	expiresAt *int64,
) QueueQRResponse {
	return QueueQRResponse{
		Action:    action,
		QueueID:   queueID,
		Type:      qrType,
		JoinURL:   joinURL,
		ExpiresAt: expiresAt,
	}
}

type JoinByTokenRequest struct {
	// Token join dari QR: format `v1.<payload>.<sig>`; JSON key-nya `t` (tautan `/join?t=...`).
	Token string `json:"t" binding:"required"`
}

type SetQRTypeRequest struct {
	Type string `json:"type" binding:"required"`
}

// NextTicketRequest: body Next, Skip, dan Recall. `counter_id` opsional (0 = counter default). `expected_ticket_id`
// opsional: nomor yang dilihat perangkat sebagai nomor counter ini (0 = counter kosong). Nil = klien lama, tanpa
// pengecekan (D9).
type NextTicketRequest struct {
	CounterID        int64  `json:"counter_id"`
	ExpectedTicketID *int64 `json:"expected_ticket_id"`
}

// SkipSettingsRequest: body PUT /queues/:id/settings/skip.
type SkipSettingsRequest struct {
	Fallback   string `json:"fallback"`
	N          int    `json:"n"`
	MaxReturns int    `json:"max_returns"`
}

type SkipSettingsResponse struct {
	Fallback   string `json:"fallback"`
	N          *int   `json:"n"`
	MaxReturns int    `json:"max_returns"`
}

func NewSkipSettingsResponse(s model.SkipSettings) SkipSettingsResponse {
	var n *int
	if s.N > 0 {
		v := s.N
		n = &v
	}
	return SkipSettingsResponse{Fallback: s.Fallback, N: n, MaxReturns: s.MaxReturns}
}
