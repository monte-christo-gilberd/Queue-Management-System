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
	// isi http.Server dengan alamat, handler, dan ketiga timeout di atas
	return &http.Server{
		// alamat tempat server listen, mis. ":8080"
		Addr: addr,
		// handler yang melayani setiap request
		Handler: handler,
		// batas waktu membaca header request
		ReadHeaderTimeout: ReadHeaderTimeout,
		// batas waktu membaca seluruh request termasuk body
		ReadTimeout: ReadTimeout,
		// batas waktu koneksi keep-alive yang menganggur
		IdleTimeout: IdleTimeout,
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
	// channel untuk menerima hasil ListenAndServe dari goroutine; buffer 1 agar goroutine tidak macet
	serveErr := make(chan error, 1)
	// jalankan server di goroutine terpisah agar fungsi ini bisa menunggu sinyal berhenti
	go func() {
		// ListenAndServe memblokir sampai server berhenti; hasilnya dikirim ke serveErr
		serveErr <- srv.ListenAndServe()
	}()

	// tunggu salah satu dari dua kejadian: server berhenti sendiri, atau ctx dibatalkan
	select {
	// server berhenti sendiri (mis. port sudah dipakai). ErrServerClosed hanya muncul setelah Shutdown.
	case err := <-serveErr:
		// ErrServerClosed berarti server ditutup dengan benar: anggap sukses
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		// error lain (misalnya port sudah dipakai): kembalikan error itu
		return err
	// ctx dibatalkan (mis. SIGTERM): lanjut ke proses shutdown di bawah
	case <-ctx.Done():
	}

	// onShutdown diisi (tidak nil): panggil dulu untuk menutup koneksi berumur panjang seperti SSE
	if onShutdown != nil {
		onShutdown()
	}

	// batasi waktu shutdown sampai shutdownTimeout; context baru dipakai karena ctx sudah dibatalkan
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	// lepaskan resource timer context saat fungsi selesai
	defer cancel()

	// tunggu request yang sedang berjalan selesai; jika waktu habis (error), paksa tutup semua koneksi
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// tutup paksa sisa koneksi; error dari Close diabaikan karena kita sudah mengembalikan error utama
		_ = srv.Close()
		// bungkus error dengan keterangan batas waktu shutdown
		return fmt.Errorf("graceful shutdown tidak selesai dalam %s: %w", shutdownTimeout, err)
	}

	// ambil hasil dari goroutine ListenAndServe; ErrServerClosed normal setelah Shutdown, jadi diabaikan
	// error lain tetap dikembalikan
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	// shutdown bersih tanpa error
	return nil
}
