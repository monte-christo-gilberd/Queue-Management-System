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
	// QueueID: id antrian. Juga kunci utama baris ini.
	QueueID int64 `db:"queue_id"`
	// QRType: jenis QR yang aktif: static atau dynamic.
	QRType string `db:"qr_type"`
	// QRCurrentCode: token statis terakhir. Tidak valid (null) bila belum pernah diterbitkan.
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
	// Fallback: ke mana nomor yang dilewati pergi: remove, back, atau after_n.
	Fallback string `db:"skip_fallback"`
	// N: posisi kembali untuk after_n (mis. 3 = masuk di urutan ke-4). 0 untuk fallback lain.
	N int `db:"skip_fallback_n"`
	// MaxReturns: berapa kali satu tiket boleh dikembalikan sebelum menjadi skipped permanen.
	MaxReturns int `db:"skip_max_returns"`
}

// DefaultSkipSettings: perilaku sebelum pengaturan ada (nomor yang dilewati tidak kembali).
func DefaultSkipSettings() SkipSettings {
	// Nomor yang dilewati dikeluarkan (remove) dan boleh dikembalikan sekali (MaxReturns 1).
	return SkipSettings{Fallback: SkipFallbackRemove, MaxReturns: 1}
}
