package model

import "encoding/json"

// OutboxItem: satu email yang diklaim dari outbox. Attempts sudah termasuk percobaan ini.
type OutboxItem struct {
	// ID: id baris outbox.
	ID int64
	// Kind: jenis email, sekaligus nama templatnya (mis. ticket-called).
	Kind string
	// Email: alamat penerima, diambil dari tabel users.
	Email string
	// Attempts: jumlah percobaan termasuk percobaan ini.
	Attempts int
	// Payload: data JSON dari baris outbox (nomor, nama antrian, dll.).
	Payload json.RawMessage
}

// OutboxPayload: data yang dibaca dari kolom payload baris outbox. Dibentuk oleh trigger database.
type OutboxPayload struct {
	// QueueID: id antrian. Wajib tidak nol.
	QueueID int64 `json:"queue_id"`
	// QueueName: nama antrian. Wajib tidak kosong.
	QueueName string `json:"queue_name"`
	// Prefix: prefix nomor tiket.
	Prefix string `json:"prefix"`
	// Number: nomor tiket tanpa prefix.
	Number int `json:"number"`
	// Name: nama penerima untuk sapaan di email.
	Name string `json:"name"`
}
