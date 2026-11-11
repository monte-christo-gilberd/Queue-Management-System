package pusher_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/pusher"
)

func TestRetryStopsOnSuccess(t *testing.T) {
	n := 0
	err := pusher.Retry(context.Background(), 3, time.Millisecond, func() error {
		n++
		if n < 3 {
			return apperror.ErrTemporaryPush
		}
		return nil
	})
	if err != nil || n != 3 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryGivesUp(t *testing.T) {
	n := 0
	err := pusher.Retry(context.Background(), 3, time.Millisecond, func() error { n++; return apperror.ErrTemporaryPush })
	if !errors.Is(err, apperror.ErrTemporaryPush) || n != 3 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryNoRepeatOnPermanent(t *testing.T) {
	n := 0
	err := pusher.Retry(context.Background(), 3, time.Millisecond, func() error { n++; return apperror.ErrSubscriptionGone })
	if err != apperror.ErrSubscriptionGone || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRetryStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := pusher.Retry(ctx, 3, time.Hour, func() error { return apperror.ErrTemporaryPush })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
