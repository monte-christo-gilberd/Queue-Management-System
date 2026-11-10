package push

import (
	"context"
	"errors"
	"time"
)

// ErrTemporary menandai kegagalan sementara (jaringan, HTTP 429/5xx) yang layak dicoba ulang.
var ErrTemporary = errors.New("temporary push failure")

// Retry menjalankan fn hingga attempts kali dengan jeda berlipat (base, 2*base, ...).
// Hanya ErrTemporary yang diulang; error lain (termasuk ErrGone) langsung dikembalikan.
func Retry(ctx context.Context, attempts int, base time.Duration, fn func() error) error {
	// Simpan galat dari percobaan terakhir untuk dikembalikan.
	var err error
	// Ulangi sampai jumlah percobaan habis.
	for i := 0; i < attempts; i++ {
		// Berhasil, atau galat bukan ErrTemporary (tidak layak diulang): kembalikan hasilnya sekarang.
		if err = fn(); err == nil || !errors.Is(err, ErrTemporary) {
			return err
		}
		// Ini percobaan terakhir: tidak perlu menunggu lagi, keluar dari loop.
		if i == attempts-1 {
			break
		}
		// Tunggu sebelum mencoba lagi, dengan jeda berlipat (base, 2*base, ...). Konteks batal menghentikan tunggu.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(base << i):
		}
	}
	// Semua percobaan habis dengan galat sementara: kembalikan galat terakhir.
	return err
}
