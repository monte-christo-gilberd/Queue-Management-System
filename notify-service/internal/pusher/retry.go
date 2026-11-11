package pusher

import (
	"context"
	"errors"
	"time"

	"qms/notify-service/internal/apperror"
)

// Retry menjalankan fn hingga attempts kali dengan jeda berlipat (base, 2*base, ...).
// Hanya apperror.ErrTemporaryPush yang diulang; error lain (termasuk ErrSubscriptionGone) langsung dikembalikan.
func Retry(
	ctx context.Context,
	attempts int,
	base time.Duration,
	fn func() error,
) error {
	// Simpan galat dari percobaan terakhir untuk dikembalikan.
	var err error
	// Ulangi sampai jumlah percobaan habis.
	for i := 0; i < attempts; i++ {
		// Berhasil, atau galat bukan apperror.ErrTemporaryPush (tidak layak diulang): kembalikan hasilnya sekarang.
		if err = fn(); err == nil || !errors.Is(err, apperror.ErrTemporaryPush) {
			return err
		}
		// Ini percobaan terakhir: tidak perlu menunggu lagi, keluar dari loop.
		if i == attempts-1 {
			break
		}
		// Tunggu sebelum mencoba lagi, dengan jeda berlipat (base, 2*base, ...). Konteks batal menghentikan tunggu.
		select {
		// Konteks batal: berhenti menunggu dan kembalikan galat konteks.
		case <-ctx.Done():
			return ctx.Err()
		// Jeda selesai: lanjut ke percobaan berikutnya.
		case <-time.After(base << i):
		}
	}
	// Semua percobaan habis dengan galat sementara: kembalikan galat terakhir.
	return err
}
