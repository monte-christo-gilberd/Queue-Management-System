package httpserver

import (
	"net/http"
	"time"
)

const (
	// ReadHeaderTimeout membatasi waktu membaca header request (melawan klien yang lambat).
	ReadHeaderTimeout = 5 * time.Second
	// ReadTimeout membatasi waktu membaca seluruh request.
	ReadTimeout = 10 * time.Second
	// WriteTimeout membatasi waktu menulis respons.
	WriteTimeout = 15 * time.Second
	// IdleTimeout membatasi koneksi keep-alive yang menganggur.
	IdleTimeout = 60 * time.Second
)

// NewServer membungkus handler dengan timeout yang wajar agar koneksi lambat tidak menggantung selamanya.
func NewServer(
	addr string,
	handler http.Handler,
) *http.Server {
	// susun server dengan alamat, handler, dan batas waktu baca, tulis, dan koneksi diam
	return &http.Server{
		// alamat dan port tempat server mendengarkan
		Addr: addr,
		// router yang menangani semua request
		Handler: handler,
		// batas waktu membaca header request
		ReadHeaderTimeout: ReadHeaderTimeout,
		// batas waktu membaca seluruh request
		ReadTimeout: ReadTimeout,
		// batas waktu menulis respons
		WriteTimeout: WriteTimeout,
		// batas koneksi keep-alive yang diam
		IdleTimeout: IdleTimeout,
	}
}
