//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"qms/internal/apperror"
	"qms/internal/database"
	"qms/internal/model"
	"qms/internal/repository"
	"qms/internal/service"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dijalankan: go test -tags integration ./internal/repository_test
// Semua query berjalan di schema sementara (qmstest_<nanodetik>) yang dibuat dari
// migrations/01_Schema.sql lalu di-DROP; data di schema asli tidak disentuh.
func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	_ = godotenv.Load("../../.env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL tidak di-set")
	}

	ctx := context.Background()
	admin, err := sqlx.Connect("pgx", dsn)
	require.NoError(t, err, "gagal connect ke DATABASE_URL (cek host/port/kredensial di backend/.env; "+
		"variabel environment DATABASE_URL di shell menimpa .env)")

	schema := fmt.Sprintf("qmstest_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err, "user DB butuh hak CREATE pada database untuk membuat schema sementara")
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := sqlx.Connect("pgx", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ddl, err := os.ReadFile("../../migrations/01_Schema.sql")
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	// Exec pgx tanpa argumen memakai simple protocol, jadi file multi-statement boleh.
	err = conn.Raw(func(driverConn interface{}) error {
		_, err := driverConn.(*stdlib.Conn).Conn().Exec(ctx, string(ddl))
		return err
	})
	require.NoError(t, err)

	return db
}

func seedUser(t *testing.T, db *sqlx.DB, email string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.Get(&id,
		`INSERT INTO users(email, username, password_hash) VALUES ($1, 'u', 'h') RETURNING id`, email))
	return id
}

func seedOwner(t *testing.T, db *sqlx.DB, email string, expires time.Time) int64 {
	t.Helper()
	id := seedUser(t, db, email)
	_, err := db.Exec(`INSERT INTO owners(user_id, subscription_expires_at) VALUES ($1, $2)`, id, expires)
	require.NoError(t, err)
	return id
}

func createQueue(t *testing.T, db *sqlx.DB, ownerID int64, name string) (model.Queue, model.QueueAuthz) {
	t.Helper()
	repo := repository.NewQueueRepository(db)
	var q model.Queue
	var a model.QueueAuthz
	err := database.WithTx(context.Background(), db, func(tx *sqlx.Tx) error {
		var err error
		q, a, err = repo.CreateNewQueue(context.Background(), tx, ownerID, name, "desc")
		return err
	})
	require.NoError(t, err)
	return q, a
}

func joinTicket(t *testing.T, db *sqlx.DB, queueID, userID int64) model.Ticket {
	t.Helper()
	repo := repository.NewTicketRepository(db)
	var tk model.Ticket
	err := database.WithTx(context.Background(), db, func(tx *sqlx.Tx) error {
		max, err := repo.MaxWaitingSortKey(context.Background(), tx, queueID)
		if err != nil {
			return err
		}
		tk, err = repo.Insert(context.Background(), tx, queueID, userID, max+65536)
		return err
	})
	require.NoError(t, err)
	return tk
}

func TestUserRepository(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	u, err := repo.Create(ctx, "a@b.c", "abc", "hash")
	require.NoError(t, err)
	assert.NotZero(t, u.ID)
	assert.Equal(t, "hash", u.PasswordHash.String)
	assert.False(t, u.GoogleID.Valid)

	_, err = repo.Create(ctx, "a@b.c", "other", "hash")
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	got, err := repo.FindByEmail(ctx, "a@b.c")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)

	_, err = repo.FindByEmail(ctx, "nobody@b.c")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestSubscriptionChecker(t *testing.T) {
	db := newTestDB(t)
	checker := repository.NewSubscriptionChecker(db)
	ctx := context.Background()

	active := seedOwner(t, db, "active@b.c", time.Now().Add(24*time.Hour))
	expired := seedOwner(t, db, "expired@b.c", time.Now().Add(-24*time.Hour))
	notOwner := seedUser(t, db, "plain@b.c")

	for id, want := range map[int64]bool{active: true, expired: false, notOwner: false, 999999: false} {
		got, err := checker.IsActive(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, want, got, "owner %d", id)
	}
}

func TestQueueRepository(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewQueueRepository(db)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))

	q1, authz := createQueue(t, db, owner, "Q1")
	assert.Equal(t, "active", q1.Status)
	assert.Equal(t, q1.ID, authz.QueueID)
	assert.True(t, authz.CanViewQueue)
	assert.False(t, authz.CanManageQueue)

	got, gotAuthz, err := repo.GetQueueByID(ctx, q1.ID)
	require.NoError(t, err)
	assert.Equal(t, "Q1", got.QueueName)
	assert.Equal(t, "desc", got.QueueDesc.String)
	assert.False(t, got.ClosedAt.Valid)
	assert.Equal(t, authz, gotAuthz)

	_, _, err = repo.GetQueueByID(ctx, 999999)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	// UpdateQueueDetails: string kosong = pertahankan nilai lama.
	require.NoError(t, repo.UpdateQueueDetails(ctx, q1.ID, "Q1 baru", ""))
	got, _, _ = repo.GetQueueByID(ctx, q1.ID)
	assert.Equal(t, "Q1 baru", got.QueueName)
	assert.Equal(t, "desc", got.QueueDesc.String)
	assert.ErrorIs(t, repo.UpdateQueueDetails(ctx, 999999, "x", "y"), apperror.ErrNotFound)

	// UpdateQueueAuthz: semantik toggle.
	fresh, err := repo.UpdateQueueAuthz(ctx, q1.ID, model.QueueAuthz{CanManageQueue: true, CanViewQueue: true})
	require.NoError(t, err)
	assert.True(t, fresh.CanManageQueue)
	assert.False(t, fresh.CanViewQueue) // true -> dibalik jadi false
	fresh, err = repo.UpdateQueueAuthz(ctx, q1.ID, model.QueueAuthz{CanManageQueue: true})
	require.NoError(t, err)
	assert.False(t, fresh.CanManageQueue)
	noop, err := repo.UpdateQueueAuthz(ctx, q1.ID, model.QueueAuthz{})
	require.NoError(t, err)
	assert.Equal(t, fresh, noop)
	_, err = repo.UpdateQueueAuthz(ctx, 999999, model.QueueAuthz{CanManageQr: true})
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	_, err = repo.UpdateQueueAuthz(ctx, 999999, model.QueueAuthz{})
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	// ListQueueByOwner + CloseQueue.
	q2, _ := createQueue(t, db, owner, "Q2")
	require.NoError(t, repo.CloseQueue(ctx, q1.ID))
	assert.ErrorIs(t, repo.CloseQueue(ctx, q1.ID), apperror.ErrNotFound) // sudah closed
	assert.ErrorIs(t, repo.CloseQueue(ctx, 999999), apperror.ErrNotFound)

	all, err := repo.ListQueueByOwner(ctx, owner, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, q2.ID, all[0].ID) // created_at DESC

	activeOnly, err := repo.ListQueueByOwner(ctx, owner, true)
	require.NoError(t, err)
	require.Len(t, activeOnly, 1)
	assert.Equal(t, q2.ID, activeOnly[0].ID)

	none, err := repo.ListQueueByOwner(ctx, 999999, false)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestTicketRepository(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewTicketRepository(db)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	q, _ := createQueue(t, db, owner, "Q")
	other, _ := createQueue(t, db, owner, "Other")
	u1, u2, u3 := seedUser(t, db, "1@b.c"), seedUser(t, db, "2@b.c"), seedUser(t, db, "3@b.c")

	// MaxWaitingSortKey: queue kosong = 0.
	var max int64
	require.NoError(t, database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		var err error
		max, err = repo.MaxWaitingSortKey(ctx, tx, q.ID)
		return err
	}))
	assert.Equal(t, int64(0), max)

	t1 := joinTicket(t, db, q.ID, u1)
	t2 := joinTicket(t, db, q.ID, u2)
	t3 := joinTicket(t, db, q.ID, u3)
	joinTicket(t, db, other.ID, u1)
	assert.Equal(t, []int{1, 2, 3}, []int{t1.TicketNumber, t2.TicketNumber, t3.TicketNumber})
	assert.Equal(t, "waiting", t1.Status)
	assert.Less(t, t1.SortKey, t2.SortKey)

	// Insert: user yang sama di queue yang sama -> ErrDuplicate (UNIQUE queue_id,user_id).
	err := database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		_, err := repo.Insert(ctx, tx, q.ID, u1, 999999999)
		return err
	})
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	// HasTicket.
	for _, c := range []struct {
		queue, user int64
		want        bool
	}{{q.ID, u1, true}, {other.ID, u2, false}, {q.ID, 999999, false}, {999999, u1, false}} {
		got, err := repo.HasTicket(ctx, c.queue, c.user)
		require.NoError(t, err)
		assert.Equal(t, c.want, got, "queue=%d user=%d", c.queue, c.user)
	}

	// NextWaiting + MarkCompleted.
	var next model.Ticket
	require.NoError(t, database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		var err error
		next, err = repo.NextWaiting(ctx, tx, q.ID)
		if err != nil {
			return err
		}
		return repo.MarkCompleted(ctx, tx, next.ID)
	}))
	assert.Equal(t, t1.ID, next.ID)
	err = database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		return repo.MarkCompleted(ctx, tx, 999999)
	})
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	err = database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		_, err := repo.NextWaiting(ctx, tx, 999999)
		return err
	})
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	// ListByQueue: tanpa filter, filter satu/dua status, status tanpa data, queue lain/kosong.
	all, err := repo.ListByQueue(ctx, q.ID, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{all[0].TicketNumber, all[1].TicketNumber, all[2].TicketNumber})
	assert.Equal(t, "completed", all[0].Status)
	assert.True(t, all[0].CompletedAt.Valid)
	assert.False(t, all[1].CompletedAt.Valid)

	waiting, err := repo.ListByQueue(ctx, q.ID, []string{"waiting"})
	require.NoError(t, err)
	assert.Len(t, waiting, 2)

	two, err := repo.ListByQueue(ctx, q.ID, []string{"waiting", "completed"})
	require.NoError(t, err)
	assert.Len(t, two, 3)

	cancelled, err := repo.ListByQueue(ctx, q.ID, []string{"cancelled"})
	require.NoError(t, err)
	assert.NotNil(t, cancelled)
	assert.Empty(t, cancelled)

	empty, err := repo.ListByQueue(ctx, 999999, nil)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	onlyOther, err := repo.ListByQueue(ctx, other.ID, nil)
	require.NoError(t, err)
	assert.Len(t, onlyOther, 1)
}

// Alur nyata: service + repo + constraint UNIQUE (queue_id, user_id) di Postgres.
func TestJoinQueue_TwiceBySameUserIsAlreadyJoined(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	customer := seedUser(t, db, "c@b.c")
	q, _ := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		database.NewSqlxTxRunner(db),
	)
	svc := service.NewTicketService(ticketRepo, database.NewSqlxTxRunner(db)).WithQueueGuard(queueSvc)

	first, err := svc.JoinQueue(ctx, q.ID, customer)
	require.NoError(t, err)
	assert.Equal(t, 1, first.TicketNumber)

	_, err = svc.JoinQueue(ctx, q.ID, customer)
	assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)

	// Setelah tiket selesai pun user tetap tidak bisa join ulang (UNIQUE tidak melihat status).
	_, err = svc.NextTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	_, err = svc.JoinQueue(ctx, q.ID, customer)
	assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)

	// Queue closed -> ditolak sebelum menyentuh tiket.
	require.NoError(t, repository.NewQueueRepository(db).CloseQueue(ctx, q.ID))
	other := seedUser(t, db, "d@b.c")
	_, err = svc.JoinQueue(ctx, q.ID, other)
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)

	// Non-owner tidak boleh Next.
	q2, _ := createQueue(t, db, owner, "Q2")
	_, err = svc.JoinQueue(ctx, q2.ID, customer)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q2.ID, customer)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestQueueStatus_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	u1, u2, u3 := seedUser(t, db, "1@b.c"), seedUser(t, db, "2@b.c"), seedUser(t, db, "3@b.c")
	outsider := seedUser(t, db, "x@b.c")
	q, _ := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		txRunner,
	).WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)

	// Belum ada tiket: owner melihat status kosong; orang luar 404.
	st, err := svc.QueueStatus(ctx, q.ID, owner)
	require.NoError(t, err)
	assert.Nil(t, st.MyTicket)
	assert.Nil(t, st.CurrentNumber)
	assert.Nil(t, st.WaitingAhead)
	_, err = svc.QueueStatus(ctx, q.ID, outsider)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	t1, err := svc.JoinQueue(ctx, q.ID, u1)
	require.NoError(t, err)
	_, err = svc.JoinQueue(ctx, q.ID, u2)
	require.NoError(t, err)
	t3, err := svc.JoinQueue(ctx, q.ID, u3)
	require.NoError(t, err)

	// u3 antrean ketiga: 2 orang di depan, belum ada yang dilayani.
	st, err = svc.QueueStatus(ctx, q.ID, u3)
	require.NoError(t, err)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, t3.ID, st.MyTicket.ID)
	assert.Nil(t, st.CurrentNumber)
	require.NotNil(t, st.WaitingAhead)
	assert.Equal(t, 2, *st.WaitingAhead)

	// Owner memanggil Next dua kali: nomor saat ini = tiket ke-2 (completed terbaru).
	_, err = svc.NextTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q.ID, owner)
	require.NoError(t, err)

	st, err = svc.QueueStatus(ctx, q.ID, u3)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)
	require.NotNil(t, st.WaitingAhead)
	assert.Equal(t, 0, *st.WaitingAhead)

	// u1 sudah dilayani: tiketnya completed, tidak ada hitungan di depan.
	st, err = svc.QueueStatus(ctx, q.ID, u1)
	require.NoError(t, err)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, t1.ID, st.MyTicket.ID)
	assert.Equal(t, "completed", st.MyTicket.Status)
	assert.Nil(t, st.WaitingAhead)

	// Repo langsung: FindByUser tanpa tiket -> ErrNotFound.
	_, err = ticketRepo.FindByUser(ctx, q.ID, outsider)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}
