package dto

// Message: isi satu notifikasi push. Dikirim sebagai JSON ke browser.
type Message struct {
	// Title: judul notifikasi, berisi nama antrian.
	Title string `json:"title"`
	// Body: isi notifikasi yang ditampilkan ke customer.
	Body string `json:"body"`
	// URL: alamat yang dibuka saat notifikasi diklik, mis. /ticket/12.
	URL string `json:"url"`
}
