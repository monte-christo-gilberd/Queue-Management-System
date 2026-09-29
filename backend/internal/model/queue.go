package model

import (
	"database/sql"
	"time"
)

type Queue struct {
	ID        int64          `db:"id"`
	OwnerID   int64          `db:"owner_id"`
	QueueName string         `db:"queue_name"`
	QueueDesc sql.NullString `db:"queue_desc"`
	Status    string         `db:"status"`
	CreatedAt time.Time      `db:"created_at"`
	ClosedAt  sql.NullTime   `db:"closed_at"`
}

type QueueAuthz struct {
	QueueID           int64 `db:"queue_id"`
	CanManageQueue    bool `db:"can_manage_queue"`
	CanManageCounters bool `db:"can_manage_counters"`
	CanManageQr       bool `db:"can_manage_qr"`
	CanViewQueue      bool `db:"can_view_queue"`
	CanManageDisplay  bool `db:"can_manage_display"`
	CanContinueQueue  bool `db:"can_continue_queue"`
	CanNotifyCustomer bool `db:"can_notify_customer"`
	CanManageTickets  bool `db:"can_manage_tickets"`
	CanManageNotes    bool `db:"can_manage_notes"`
}
