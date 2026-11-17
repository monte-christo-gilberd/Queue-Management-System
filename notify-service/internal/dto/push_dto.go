package dto

// Message: isi satu notifikasi push. Web Push mengirimnya sebagai JSON ke browser; FCM memetakannya ke notification (title, body) dan data.url.
type Message struct {
	// Title: judul notifikasi, berisi nama antrian.
	Title string `json:"title"`
	// Body: isi notifikasi yang ditampilkan ke customer.
	Body string `json:"body"`
	// URL: alamat yang dibuka saat notifikasi diklik, mis. /ticket/12, dengan 12 adalah id antrian (queue_id), bukan id tiket.
	URL string `json:"url"`
}
