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
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil || !errors.Is(err, ErrTemporary) {
			return err
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(base << i):
		}
	}
	return err
}
