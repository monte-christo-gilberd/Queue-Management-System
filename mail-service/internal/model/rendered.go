package model

// Rendered: isi satu email dari tiga berkas templat: <nama>.subject.tmpl, <nama>.txt.tmpl, <nama>.html.tmpl.
type Rendered struct {
	// Subject: subjek satu baris, sudah dibersihkan dari CR dan LF.
	Subject string
	// Text: isi versi teks biasa.
	Text string
	// HTML: isi versi HTML, dibungkus layout.html.tmpl.
	HTML string
}
