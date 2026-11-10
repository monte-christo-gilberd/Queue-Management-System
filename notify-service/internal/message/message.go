package message

import "fmt"

// Label membuat label nomor tiket untuk teks notifikasi, mis. "A-007". Ini satu-satunya tempat format nomor itu.
// Prefix kosong dianggap "A".
func Label(prefix string, number int) string {
	// Prefix kosong: pakai "A" sebagai awalan bawaan.
	if prefix == "" {
		prefix = "A"
	}
	// Gabungkan prefix dan nomor dengan tiga digit (dengan nol di depan), mis. A-007.
	return fmt.Sprintf("%s-%03d", prefix, number)
}

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

// Subscription: satu langganan push milik browser, yaitu tujuan dan kunci enkripsinya.
type Subscription struct {
	// Endpoint: alamat layanan push milik browser.
	Endpoint string
	// P256dh: kunci publik ECDH langganan (base64url).
	P256dh string
	// Auth: secret autentikasi langganan (base64url).
	Auth string
}

// Message: isi satu notifikasi push. Dikirim sebagai JSON ke browser.
type Message struct {
	// Title: judul notifikasi, berisi nama antrian.
	Title string `json:"title"`
	// Body: isi notifikasi yang ditampilkan ke customer.
	Body string `json:"body"`
	// URL: alamat yang dibuka saat notifikasi diklik, mis. /ticket/12.
	URL string `json:"url"`
}
