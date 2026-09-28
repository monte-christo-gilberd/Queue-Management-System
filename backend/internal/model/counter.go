package model

import (
	"database/sql"
	"time"
)

type Counter struct {
	ID               int64         `db:"id"`
	QueueID          int64         `db:"queue_id"`
	Name             string        `db:"name"`
	AttendingAdminID sql.NullInt64 `db:"attending_admin_id"`
	AttendedTicketID sql.NullInt64 `db:"attended_ticket_id"`
	CreatedAt        time.Time     `db:"created_at"`
}
