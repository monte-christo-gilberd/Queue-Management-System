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

// Nilai queue_settings.skip_fallback.
const (
	SkipFallbackRemove = "remove"
	SkipFallbackBack   = "back"
	SkipFallbackAfterN = "after_n"
)

// SkipSettings: apa yang terjadi pada nomor yang dilewati. N hanya berarti untuk after_n (0 = tidak ada).
// MaxReturns = berapa kali satu tiket boleh dikembalikan sebelum menjadi skipped permanen.
type SkipSettings struct {
	Fallback   string `db:"skip_fallback"`
	N          int    `db:"skip_fallback_n"`
	MaxReturns int    `db:"skip_max_returns"`
}

// DefaultSkipSettings: perilaku sebelum pengaturan ada (nomor yang dilewati tidak kembali).
func DefaultSkipSettings() SkipSettings {
	return SkipSettings{Fallback: SkipFallbackRemove, MaxReturns: 1}
}
