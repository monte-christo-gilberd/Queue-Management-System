package model

// Message: satu email. Text wajib; HTML opsional (bila ada, dikirim sebagai multipart/alternative).
type Message struct {
	// To: alamat penerima.
	To string
	// Subject: subjek email. Tidak boleh berisi CR atau LF.
	Subject string
	// Text: isi versi teks biasa. Wajib tidak kosong.
	Text string
	// HTML: isi versi HTML. Opsional.
	HTML string
}
