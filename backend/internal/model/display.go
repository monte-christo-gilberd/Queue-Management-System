package model

import (
	"database/sql"
	"time"
)

// QueueDisplay: konfigurasi layar publik satu antrian. Token ada di URL layar.
// Dipakai oleh DisplayRepository.Get dan DisplayService.Config (GET /queues/:queue_id/display),
// DisplayRepository.Upsert dan DisplayService.Configure (PUT /queues/:queue_id/display),
// DisplayRepository.Rotate dan DisplayService.Rotate (POST /queues/:queue_id/display/rotate),
// serta DisplayRepository.GetByToken (GET /display/:token).
type QueueDisplay struct {
	// QueueID: id antrian yang ditampilkan layar ini.
	QueueID int64 `db:"queue_id"`
	// Token: token acak sebagai kunci akses layar publik tanpa login.
	Token string `db:"token"`
	// UpcomingCount: jumlah maksimum nomor yang belum dipanggil dan ditampilkan.
	UpcomingCount int `db:"upcoming_count"`
	// SkippedCount: jumlah maksimum nomor yang dilewati dan ditampilkan.
	SkippedCount int `db:"skipped_count"`
	// CreatedAt: waktu konfigurasi layar dibuat.
	CreatedAt time.Time `db:"created_at"`
	// RotatedAt: waktu token terakhir diganti. Tidak valid (null) bila belum pernah diganti. Token di baris ini selalu yang berlaku.
	RotatedAt sql.NullTime `db:"rotated_at"`
}

// DisplayQueueInfo: kolom antrian yang boleh tampil di layar publik.
// Dipakai oleh DisplayRepository.QueueInfo (GET /display/:token).
type DisplayQueueInfo struct {
	// Name: nama antrian.
	Name string `db:"queue_name"`
	// Desc: deskripsi antrian. Tidak valid (null) bila tidak diisi.
	Desc sql.NullString `db:"queue_desc"`
	// Prefix: prefix nomor tiket, mis. "A" di "A-007".
	Prefix string `db:"ticket_prefix"`
	// Status: status antrian: active atau closed.
	Status string `db:"status"`
}

// DisplayCounter: satu kartu di layar. Serving berisi label siap tampil ("A-007"), kosong bila counter tidak melayani.
// Dipakai oleh DisplayService.build.
type DisplayCounter struct {
	// ID: id counter.
	ID int64
	// Name: nama counter yang ditampilkan di layar.
	Name string
	// Serving: label nomor yang sedang dilayani counter ini (mis. "A-007"). Kosong bila counter tidak melayani.
	Serving string
}

// DisplaySnapshot: keadaan antrian untuk layar publik. Sengaja hanya berisi teks siap tampil: tanpa id tiket,
// id user, nama customer, atau catatan owner.
// Dipakai oleh DisplayService.build (GET /display/:token).
type DisplaySnapshot struct {
	// QueueName: nama antrian yang tampil di layar.
	QueueName string
	// QueueDesc: deskripsi antrian. Nil bila tidak diisi.
	QueueDesc *string
	// Status: status antrian: active atau closed.
	Status string
	// MultiCounter: true bila antrian punya lebih dari satu counter, sehingga layar menampilkan kartu counter.
	MultiCounter bool
	// Counters: daftar kartu counter.
	Counters []DisplayCounter
	// Upcoming: label nomor yang sedang menunggu, urut dari yang paling depan.
	Upcoming []string
	// Skipped: label nomor yang dilewati.
	Skipped []string
	// GeneratedAt: waktu snapshot ini dibuat.
	GeneratedAt time.Time
}
