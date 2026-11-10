package model

import (
	"database/sql"
	"time"
)

// Queue: satu baris tabel queues, yaitu antrian yang dibuat owner.
type Queue struct {
	// ID: id antrian di database.
	ID int64 `db:"id"`
	// OwnerID: id user yang memiliki antrian ini.
	OwnerID int64 `db:"owner_id"`
	// QueueName: nama antrian yang tampil di layar dan daftar.
	QueueName string `db:"queue_name"`
	// QueueDesc: deskripsi antrian. Tidak valid (null) bila owner tidak mengisinya.
	QueueDesc sql.NullString `db:"queue_desc"`
	// TicketPrefix: prefix nomor tiket, mis. "A" di "A-007".
	TicketPrefix string `db:"ticket_prefix"`
	// Status: status antrian: active (menerima tiket) atau closed (sudah ditutup).
	Status string `db:"status"`
	// CreatedAt: waktu antrian dibuat.
	CreatedAt time.Time `db:"created_at"`
	// ClosedAt: waktu antrian ditutup. Tidak valid (null) selama antrian masih aktif.
	ClosedAt sql.NullTime `db:"closed_at"`
}
