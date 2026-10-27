package model

import "database/sql"

// QueueQRTypeStatic dan QueueQRTypeDynamic adalah nilai tetap kolom
// queue_settings.qr_type (CHECK constraint di schema: static | dynamic).
const (
	QueueQRTypeStatic  = "static"
	QueueQRTypeDynamic = "dynamic"
)

// QueueSettings: satu baris konfigurasi QR per antrian (primary key = queue_id).
// qr_current_code menyimpan token statis terakhir yang diterbitkan.
type QueueSettings struct {
	QueueID       int64          `db:"queue_id"`
	QRType        string         `db:"qr_type"`
	QRCurrentCode sql.NullString `db:"qr_current_code"`
}
