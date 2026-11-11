package httpserver

import (
	"net/http"
	"time"
)

const (
	// ReadHeaderTimeout membatasi waktu membaca header request; menahan koneksi yang mengirim header sangat lambat.
	ReadHeaderTimeout = 10 * time.Second
	// ReadTimeout membatasi waktu membaca seluruh request termasuk body.
	ReadTimeout = 30 * time.Second
	// WriteTimeout membatasi waktu menulis respons.
	WriteTimeout = 30 * time.Second
	// IdleTimeout membatasi koneksi keep-alive yang menganggur.
	IdleTimeout = 60 * time.Second
)

// New membuat http.Server untuk addr (mis. ":8094") dan handler, dengan batas waktu agar koneksi macet tidak menggantung.
func New(
	addr string,
	handler http.Handler,
) *http.Server {
	// isi http.Server dengan alamat, handler, dan keempat timeout di atas
	return &http.Server{
		// alamat tempat server listen
		Addr: addr,
		// handler yang melayani setiap request
		Handler: handler,
		// batas waktu membaca header request
		ReadHeaderTimeout: ReadHeaderTimeout,
		// batas waktu membaca seluruh request
		ReadTimeout: ReadTimeout,
		// batas waktu menulis respons
		WriteTimeout: WriteTimeout,
		// batas waktu koneksi keep-alive yang menganggur
		IdleTimeout: IdleTimeout,
	}
}
