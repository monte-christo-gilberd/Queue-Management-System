package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"qms/payment-service/internal/billing"
)

// unreachableStore memakai pool yang menunjuk port yang pasti menolak koneksi, jadi jalur galat
// PGStore bisa diuji tanpa Postgres. Test integrasi sungguhan ada di store_integration_test.go.
func unreachableStore(t *testing.T) *billing.PGStore {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://payment_rw:x@127.0.0.1:1/qms_db?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return billing.NewPGStore(pool)
}

func TestPGStoreErrorsWhenDatabaseUnreachable(t *testing.T) {
	st := unreachableStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := st.Ping(ctx); err == nil {
		t.Error("Ping: want error")
	}
	if _, err := st.UserEmail(ctx, 1); err == nil || err == billing.ErrUnknownUser {
		t.Errorf("UserEmail: err = %v, want connection error (not ErrUnknownUser)", err)
	}
	if err := st.RecordEvent(ctx, "evt_x", "t"); err == nil {
		t.Error("RecordEvent: want error")
	}
	ok, _, err := st.Credit(ctx, billing.Credit{EventID: "evt_x", EventType: "t", SessionID: "cs_x", UserID: 1, Amount: "1.00", Currency: "IDR"})
	if err == nil || ok {
		t.Errorf("Credit: ok=%v err=%v, want failure", ok, err)
	}
}

func TestHealthReportsUnreachableDatabase(t *testing.T) {
	svc := billing.New(unreachableStore(t), nil, "https://qms.example.com", nil)
	if err := svc.Health(context.Background()); err == nil {
		t.Fatal("Health must fail when the database is unreachable")
	}
}
