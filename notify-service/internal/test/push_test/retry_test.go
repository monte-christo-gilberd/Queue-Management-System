package push_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"qms/notify-service/internal/push"
)

func TestRetryStopsOnSuccess(t *testing.T) {
	n := 0
	err := push.Retry(context.Background(), 3, time.Millisecond, func() error {
		n++
		if n < 3 {
			return push.ErrTemporary
		}
		return nil
	})
	if err != nil || n != 3 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryGivesUp(t *testing.T) {
	n := 0
	err := push.Retry(context.Background(), 3, time.Millisecond, func() error { n++; return push.ErrTemporary })
	if !errors.Is(err, push.ErrTemporary) || n != 3 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryNoRepeatOnPermanent(t *testing.T) {
	n := 0
	err := push.Retry(context.Background(), 3, time.Millisecond, func() error { n++; return push.ErrGone })
	if err != push.ErrGone || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := push.Retry(ctx, 3, time.Hour, func() error { return push.ErrTemporary })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
