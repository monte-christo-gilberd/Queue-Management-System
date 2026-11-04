package billing_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"qms/payment-service/internal/billing"
)

// Test integrasi terhadap Postgres sungguhan (skema 01 + db/migrations/05_payment_role.sh sudah diterapkan).
// Dilewati bila env tidak diisi:
//
//	PAYMENT_TEST_ADMIN_URL    superuser, hanya untuk menyiapkan/membersihkan data uji
//	PAYMENT_TEST_DATABASE_URL role payment_rw, yang dipakai service
func integrationEnv(t *testing.T) (admin *pgx.Conn, store *billing.PGStore, rw *pgxpool.Pool) {
	t.Helper()
	adminURL, rwURL := os.Getenv("PAYMENT_TEST_ADMIN_URL"), os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if adminURL == "" || rwURL == "" {
		t.Skip("PAYMENT_TEST_ADMIN_URL / PAYMENT_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	a, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(ctx) })
	pool, err := pgxpool.New(ctx, rwURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return a, billing.NewPGStore(pool), pool
}

// seedOwner membuat user+owner dengan masa langganan tertentu dan mengembalikan id-nya.
func seedOwner(t *testing.T, admin *pgx.Conn, email string, expires time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := admin.QueryRow(ctx,
		`INSERT INTO users (email, username, password_hash) VALUES ($1, 'owner', 'x') RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO owners (user_id, subscription_expires_at) VALUES ($1, $2)`, id, expires); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, `DELETE FROM payment_session_credits WHERE payment_id IN (SELECT id FROM payments WHERE owner_id=$1)`, id)
		admin.Exec(ctx, `DELETE FROM payments WHERE owner_id=$1`, id)
		admin.Exec(ctx, `DELETE FROM owners WHERE user_id=$1`, id)
		admin.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
	return id
}

func credit(owner int64, event, session string) billing.Credit {
	return billing.Credit{EventID: event, EventType: "checkout.session.completed", SessionID: session,
		OwnerID: owner, Amount: "50000.00", Currency: "IDR"}
}

func expiresAt(t *testing.T, admin *pgx.Conn, id int64) time.Time {
	t.Helper()
	var e time.Time
	if err := admin.QueryRow(context.Background(), `SELECT subscription_expires_at FROM owners WHERE user_id=$1`, id).Scan(&e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestCreditExtendsActiveSubscriptionFromItsEnd(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	end := time.Now().UTC().Add(10 * 24 * time.Hour).Truncate(time.Second)
	id := seedOwner(t, admin, "active-"+t.Name()+"@test.com", end)

	ok, got, err := store.Credit(context.Background(), credit(id, "evt_a_"+t.Name(), "cs_a_"+t.Name()))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if want := end.AddDate(0, 1, 0); got.Sub(want).Abs() > time.Second {
		t.Fatalf("expires = %v, want %v (end + 1 month)", got, want)
	}
}

func TestCreditExtendsExpiredSubscriptionFromNow(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	id := seedOwner(t, admin, "expired-"+t.Name()+"@test.com", time.Now().Add(-90*24*time.Hour))

	_, got, err := store.Credit(context.Background(), credit(id, "evt_e_"+t.Name(), "cs_e_"+t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Now().AddDate(0, 1, 0); got.Sub(want).Abs() > time.Minute {
		t.Fatalf("expires = %v, want about %v (now + 1 month, not from the old date)", got, want)
	}
}

func TestCreditRecordsPayment(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	id := seedOwner(t, admin, "rec-"+t.Name()+"@test.com", time.Now().Add(-time.Hour))
	if _, _, err := store.Credit(context.Background(), credit(id, "evt_r_"+t.Name(), "cs_r_"+t.Name())); err != nil {
		t.Fatal(err)
	}
	var amount, currency, plan, status string
	var extends time.Time
	err := admin.QueryRow(context.Background(),
		`SELECT amount::text, currency, plan_type, status, extends_subscription_to FROM payments WHERE owner_id=$1`, id).
		Scan(&amount, &currency, &plan, &status, &extends)
	if err != nil {
		t.Fatal(err)
	}
	if amount != "50000.00" || currency != "IDR" || plan != "monthly" || status != "success" ||
		!extends.Equal(expiresAt(t, admin, id)) {
		t.Fatalf("payment = %s %s %s %s %v", amount, currency, plan, status, extends)
	}
}

func TestSameEventTwiceExtendsOnce(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	id := seedOwner(t, admin, "dupev-"+t.Name()+"@test.com", time.Now().Add(-time.Hour))
	c := credit(id, "evt_d_"+t.Name(), "cs_d_"+t.Name())
	if ok, _, err := store.Credit(context.Background(), c); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	after := expiresAt(t, admin, id)
	if ok, _, err := store.Credit(context.Background(), c); err != nil || ok {
		t.Fatalf("replay: ok=%v err=%v, want not extended", ok, err)
	}
	if got := expiresAt(t, admin, id); !got.Equal(after) {
		t.Fatalf("replay moved the date: %v -> %v", after, got)
	}
}

func TestSameSessionFromTwoEventsExtendsOnce(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	id := seedOwner(t, admin, "dupsess-"+t.Name()+"@test.com", time.Now().Add(-time.Hour))
	session := "cs_s_" + t.Name()
	if ok, _, err := store.Credit(context.Background(), credit(id, "evt_s1_"+t.Name(), session)); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	after := expiresAt(t, admin, id)
	if ok, _, err := store.Credit(context.Background(), credit(id, "evt_s2_"+t.Name(), session)); err != nil || ok {
		t.Fatalf("second event, same session: ok=%v err=%v, want not extended", ok, err)
	}
	if got := expiresAt(t, admin, id); !got.Equal(after) {
		t.Fatalf("date moved: %v -> %v", after, got)
	}
}

func TestUnknownOwnerRollsBackEverything(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	c := credit(99999999, "evt_u_"+t.Name(), "cs_u_"+t.Name())
	if _, _, err := store.Credit(context.Background(), c); !errors.Is(err, billing.ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
	var n int
	admin.QueryRow(context.Background(), `SELECT count(*) FROM payment_webhook_events WHERE event_id=$1`, c.EventID).Scan(&n)
	if n != 0 {
		t.Fatal("failed credit must not leave the event recorded")
	}
}

func TestOwnerEmail(t *testing.T) {
	admin, store, _ := integrationEnv(t)
	id := seedOwner(t, admin, "mail-"+t.Name()+"@test.com", time.Now())
	if e, err := store.OwnerEmail(context.Background(), id); err != nil || e != "mail-"+t.Name()+"@test.com" {
		t.Fatalf("email=%q err=%v", e, err)
	}
	if _, err := store.OwnerEmail(context.Background(), 99999999); !errors.Is(err, billing.ErrNotOwner) {
		t.Fatalf("err = %v", err)
	}
}

// Isolasi di tingkat database: payment_rw tidak boleh menyentuh data antrian atau kolom lain.
func TestRoleCannotTouchQueueData(t *testing.T) {
	_, _, rw := integrationEnv(t)
	ctx := context.Background()
	for name, q := range map[string]string{
		"read tickets":            `SELECT 1 FROM tickets LIMIT 1`,
		"read queues":             `SELECT 1 FROM queues LIMIT 1`,
		"delete tickets":          `DELETE FROM tickets`,
		"read password hashes":    `SELECT password_hash FROM users LIMIT 1`,
		"update user email":       `UPDATE users SET email = email`,
		"update owner created_at": `UPDATE owners SET created_at = created_at`,
		"delete payments":         `DELETE FROM payments`,
		"update payments":         `UPDATE payments SET amount = 0`,
	} {
		if _, err := rw.Exec(ctx, q); err == nil {
			t.Errorf("%s: payment_rw was allowed to run %q", name, q)
		}
	}
}
