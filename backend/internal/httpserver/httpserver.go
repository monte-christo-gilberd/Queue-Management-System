// Package httpserver membungkus http.Server dengan timeout dasar dan shutdown yang rapi.
//
// Tidak ada WriteTimeout karena koneksi SSE (/queues/:id/event) berumur panjang; batas tulis untuk rute biasa
// dipegang nginx (proxy_read_timeout). Koneksi SSE ditutup lewat callback onShutdown (Hub.Shutdown), bukan lewat
// pembatalan konteks request, agar request biasa yang sedang berjalan (mis. query DB) tetap dituntaskan.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// ReadHeaderTimeout membatasi waktu membaca header request; menahan koneksi yang mengirim header sangat lambat.
	ReadHeaderTimeout = 5 * time.Second
	// ReadTimeout membatasi waktu membaca seluruh request termasuk body (cukup untuk upload foto profil).
	ReadTimeout = 30 * time.Second
	// IdleTimeout membatasi koneksi keep-alive yang menganggur.
	IdleTimeout = 120 * time.Second
)

// New membuat http.Server untuk addr (mis. ":8080") dan handler.
func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		ReadTimeout:       ReadTimeout,
		IdleTimeout:       IdleTimeout,
	}
}

// Run menjalankan srv sampai ctx dibatalkan (mis. SIGTERM) atau server gagal.
//
// Saat ctx dibatalkan: onShutdown dipanggil lebih dulu (boleh nil) untuk menutup koneksi berumur panjang, lalu
// srv.Shutdown menunggu request yang sedang berjalan paling lama shutdownTimeout. Bila waktu habis, koneksi
// yang tersisa ditutup paksa dan error dikembalikan. Penutupan normal mengembalikan nil.
func Run(
	ctx context.Context,
	srv *http.Server,
	shutdownTimeout time.Duration,
	onShutdown func(),
) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// Server berhenti sendiri (mis. port sudah dipakai). ErrServerClosed hanya muncul setelah Shutdown.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	if onShutdown != nil {
		onShutdown()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown tidak selesai dalam %s: %w", shutdownTimeout, err)
	}

	// ListenAndServe mengembalikan ErrServerClosed segera setelah Shutdown dipanggil.
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
