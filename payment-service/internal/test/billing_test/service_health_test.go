package billing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"qms/payment-service/internal/billing"
)

// deadlineStore mencatat apakah Ping dibatasi oleh deadline.
type deadlineStore struct {
	*fakeStore
	hasDeadline bool
	left        time.Duration
}

func (d *deadlineStore) Ping(ctx context.Context) error {
	if dl, ok := ctx.Deadline(); ok {
		d.hasDeadline = true
		d.left = time.Until(dl)
	}
	return d.fakeStore.Ping(ctx)
}

func TestHealthOK(t *testing.T) {
	if err := newSvc(&fakeStore{}, nil).Health(context.Background()); err != nil {
		t.Fatalf("Health = %v, want nil", err)
	}
}

func TestHealthPropagatesPingError(t *testing.T) {
	boom := errors.New("db down")
	if err := newSvc(&fakeStore{pingErr: boom}, nil).Health(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Health = %v, want %v", err, boom)
	}
}

func TestHealthUsesShortDeadline(t *testing.T) {
	ds := &deadlineStore{fakeStore: &fakeStore{}}
	svc := billing.New(ds, nil, "https://qms.example.com", nil)
	if err := svc.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ds.hasDeadline || ds.left <= 0 || ds.left > 2*time.Second {
		t.Fatalf("deadline present=%v left=%v, want <= 2s", ds.hasDeadline, ds.left)
	}
}
