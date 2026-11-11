package model

// Ticket: data tiket yang dibutuhkan untuk menyusun pesan notifikasi.
type Ticket struct {
	// ID: id tiket di database.
	ID int64
	// QueueID: id antrian tempat tiket ini berada.
	QueueID int64
	// Number: nomor tiket tanpa prefix.
	Number int
	// UserID: id customer pemilik tiket. Penerima notifikasi.
	UserID int64
	// Status: status tiket saat data dibaca.
	Status string
	// QueueName: nama antrian. Dipakai sebagai judul notifikasi.
	QueueName string
	// Prefix: prefix nomor antrian, mis. "A".
	Prefix string
	// CounterName: counter yang memanggil tiket ini. Kosong bila antrian hanya punya satu counter
	// (counter default disembunyikan, sama dengan UI) atau tiket belum dipanggil.
	CounterName string
}
