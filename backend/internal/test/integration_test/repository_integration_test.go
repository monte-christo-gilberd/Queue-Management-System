//go:build integration

package integration_test

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
// db/migrations/01_Schema.sql dan 02_Indexes.sql lalu di-DROP; data di schema asli tidak disentuh.
func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	_ = godotenv.Load("../../../.env")
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

	schemaSQL, err := os.ReadFile("../../../../db/migrations/01_Schema.sql")
	require.NoError(t, err)
	indexes, err := os.ReadFile("../../../../db/migrations/02_Indexes.sql")
	require.NoError(t, err)
	ddl := string(schemaSQL) + "\n" + string(indexes)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	// Exec pgx tanpa argumen memakai simple protocol, jadi file multi-statement boleh.
	err = conn.Raw(func(driverConn interface{}) error {
		_, err := driverConn.(*stdlib.Conn).Conn().Exec(ctx, ddl)
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

func createQueue(t *testing.T, db *sqlx.DB, ownerID int64, name string) model.Queue {
	t.Helper()
	repo := repository.NewQueueRepository(db)
	var q model.Queue
	err := database.WithTx(context.Background(), db, func(tx *sqlx.Tx) error {
		var err error
		q, err = repo.CreateNewQueue(context.Background(), tx, ownerID, name, "desc")
		return err
	})
	require.NoError(t, err)
	return q
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

	q1 := createQueue(t, db, owner, "Q1")
	assert.Equal(t, "active", q1.Status)

	got, err := repo.GetQueueByID(ctx, q1.ID)
	require.NoError(t, err)
	assert.Equal(t, "Q1", got.QueueName)
	assert.Equal(t, "desc", got.QueueDesc.String)
	assert.False(t, got.ClosedAt.Valid)

	_, err = repo.GetQueueByID(ctx, 999999)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	// UpdateQueueDetails: string kosong = pertahankan nilai lama.
	require.NoError(t, repo.UpdateQueueDetails(ctx, q1.ID, "Q1 baru", ""))
	got, _ = repo.GetQueueByID(ctx, q1.ID)
	assert.Equal(t, "Q1 baru", got.QueueName)
	assert.Equal(t, "desc", got.QueueDesc.String)
	assert.ErrorIs(t, repo.UpdateQueueDetails(ctx, 999999, "x", "y"), apperror.ErrNotFound)

	// ListQueueByOwner + CloseQueue.
	q2 := createQueue(t, db, owner, "Q2")
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

func TestQueueSettingsRepository(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewQueueRepository(db)
	ctx := context.Background()
	owner := seedOwner(t, db, "qr@b.c", time.Now().Add(time.Hour))

	q := createQueue(t, db, owner, "QR")

	// CreateNewQueue wajib membuat baris queue_settings dalam transaksi yang sama.
	settings, err := repo.GetQueueSettings(ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, settings.QRType)
	assert.False(t, settings.QRCurrentCode.Valid)

	// Token statis disimpan dan tetap sama sampai diganti.
	require.NoError(t, repo.UpdateQRCurrentCode(ctx, q.ID, "v1.token.sig"))
	settings, err = repo.GetQueueSettings(ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "v1.token.sig", settings.QRCurrentCode.String)

	// Beralih ke dinamis tidak menghapus token statis (QR cetak lama berlaku lagi bila kembali statis).
	require.NoError(t, repo.UpdateQRType(ctx, q.ID, model.QueueQRTypeDynamic))
	settings, err = repo.GetQueueSettings(ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeDynamic, settings.QRType)
	assert.Equal(t, "v1.token.sig", settings.QRCurrentCode.String)

	// Antrian lama tanpa baris queue_settings: lazy backfill harus idempotent.
	legacy := createQueue(t, db, owner, "Legacy")
	_, err = db.ExecContext(ctx, `DELETE FROM queue_settings WHERE queue_id = $1`, legacy.ID)
	require.NoError(t, err)
	_, err = repo.GetQueueSettings(ctx, legacy.ID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	require.NoError(t, repo.CreateQueueSettingsIfMissing(ctx, legacy.ID))
	require.NoError(t, repo.CreateQueueSettingsIfMissing(ctx, legacy.ID))
	settings, err = repo.GetQueueSettings(ctx, legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, settings.QRType)

	// Update tanpa baris settings = ErrNotFound (bukan panic / update diam-diam).
	_, err = db.ExecContext(ctx, `DELETE FROM queue_settings WHERE queue_id = $1`, legacy.ID)
	require.NoError(t, err)
	assert.ErrorIs(t, repo.UpdateQRType(ctx, legacy.ID, model.QueueQRTypeDynamic), apperror.ErrNotFound)
	assert.ErrorIs(t, repo.UpdateQRCurrentCode(ctx, legacy.ID, "x"), apperror.ErrNotFound)
}

func TestTicketRepository(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewTicketRepository(db)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	q := createQueue(t, db, owner, "Q")
	other := createQueue(t, db, owner, "Other")
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

	// NextWaiting + MarkServing: tiket waiting paling depan mulai dilayani.
	var next model.Ticket
	require.NoError(t, database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		var err error
		next, err = repo.NextWaiting(ctx, tx, q.ID)
		if err != nil {
			return err
		}
		return repo.MarkServing(ctx, tx, next.ID, 1)
	}))
	assert.Equal(t, t1.ID, next.ID)
	err = database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		return repo.MarkServing(ctx, tx, 999999, 1)
	})
	assert.ErrorIs(t, err, apperror.ErrNotFound)
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
	assert.Equal(t, "currently_served", all[0].Status)
	assert.False(t, all[0].CompletedAt.Valid)
	assert.False(t, all[1].CompletedAt.Valid)

	waiting, err := repo.ListByQueue(ctx, q.ID, []string{"waiting"})
	require.NoError(t, err)
	assert.Len(t, waiting, 2)

	two, err := repo.ListByQueue(ctx, q.ID, []string{"waiting", "currently_served"})
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
// Spek 4.2 / TC-05-04: join ulang menampilkan tiket yang sudah ada, tidak membuat nomor kedua.
func TestJoinQueue_TwiceBySameUserReturnsSameTicket(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	customer := seedUser(t, db, "c@b.c")
	q := createQueue(t, db, owner, "Q")

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

	again, err := svc.JoinQueue(ctx, q.ID, customer)
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, 1, again.TicketNumber)

	// Tiket yang sedang dilayani masih aktif: join ulang tetap mengembalikan tiket yang sama.
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)
	afterNext, err := svc.JoinQueue(ctx, q.ID, customer)
	require.NoError(t, err)
	assert.Equal(t, first.ID, afterNext.ID)
	assert.Equal(t, "currently_served", afterNext.Status)

	// Queue closed -> ditolak sebelum menyentuh tiket.
	require.NoError(t, repository.NewQueueRepository(db).CloseQueue(ctx, q.ID))
	other := seedUser(t, db, "d@b.c")
	_, err = svc.JoinQueue(ctx, q.ID, other)
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)

	// Non-owner tidak boleh Next.
	q2 := createQueue(t, db, owner, "Q2")
	_, err = svc.JoinQueue(ctx, q2.ID, customer)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q2.ID, customer, 0)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestQueueStatus_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	u1, u2, u3 := seedUser(t, db, "1@b.c"), seedUser(t, db, "2@b.c"), seedUser(t, db, "3@b.c")
	outsider := seedUser(t, db, "x@b.c")
	q := createQueue(t, db, owner, "Q")

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

	// Owner memanggil Next dua kali: nomor saat ini = tiket ke-2 (currently_served); tiket ke-1 completed.
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)

	st, err = svc.QueueStatus(ctx, q.ID, u3)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)
	require.NotNil(t, st.PreviousNumber)
	assert.Equal(t, 1, *st.PreviousNumber)
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

// Lookup login: email atau username, username ganda, dan email didahulukan.
func TestUserRepository_FindByLogin(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	a, err := repo.Create(ctx, "a@b.c", "budi", "hash-a")
	require.NoError(t, err)
	b, err := repo.Create(ctx, "b@b.c", "budi", "hash-b")
	require.NoError(t, err)
	// username "b@b.c" milik user lain: email-nya harus didahulukan saat login dengan "b@b.c".
	c, err := repo.Create(ctx, "c@b.c", "b@b.c", "hash-c")
	require.NoError(t, err)

	byEmail, err := repo.FindByLogin(ctx, "a@b.c")
	require.NoError(t, err)
	require.Len(t, byEmail, 1)
	assert.Equal(t, a.ID, byEmail[0].ID)

	byUsername, err := repo.FindByLogin(ctx, "budi")
	require.NoError(t, err)
	require.Len(t, byUsername, 2)
	assert.Equal(t, a.ID, byUsername[0].ID)
	assert.Equal(t, b.ID, byUsername[1].ID)

	emailFirst, err := repo.FindByLogin(ctx, "b@b.c")
	require.NoError(t, err)
	require.Len(t, emailFirst, 2)
	assert.Equal(t, b.ID, emailFirst[0].ID, "kecocokan email harus di depan kecocokan username")
	assert.Equal(t, c.ID, emailFirst[1].ID)

	none, err := repo.FindByLogin(ctx, "tidak-ada")
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

// Tiket saya lintas antrian: hanya milik user itu, dengan nama dan status antrian.
func TestTicketRepository_ListByUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	me := seedUser(t, db, "me@b.c")
	other := seedUser(t, db, "other@b.c")
	q1 := createQueue(t, db, owner, "Q1")
	q2 := createQueue(t, db, owner, "Q2")
	repo := repository.NewTicketRepository(db)

	joinTicket(t, db, q1.ID, me)
	joinTicket(t, db, q2.ID, me)
	joinTicket(t, db, q1.ID, other)

	mine, err := repo.ListByUser(ctx, me)
	require.NoError(t, err)
	require.Len(t, mine, 2)
	for _, tk := range mine {
		assert.Equal(t, me, tk.UserID)
		assert.NotEmpty(t, tk.QueueName)
		assert.Equal(t, "active", tk.QueueStatus)
	}

	none, err := repo.ListByUser(ctx, 999999)
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

// IsOwner: true hanya untuk user yang punya baris di owners; FindByID mengembalikan user itu.
func TestUserRepository_IsOwnerAndFindByID(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()
	owner := seedOwner(t, db, "own@b.c", time.Now().Add(time.Hour))
	plain := seedUser(t, db, "plain2@b.c")

	isOwner, err := repo.IsOwner(ctx, owner)
	require.NoError(t, err)
	assert.True(t, isOwner)

	isOwner, err = repo.IsOwner(ctx, plain)
	require.NoError(t, err)
	assert.False(t, isOwner)

	u, err := repo.FindByID(ctx, plain)
	require.NoError(t, err)
	assert.Equal(t, "plain2@b.c", u.Email)

	_, err = repo.FindByID(ctx, 999999)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

// Regresi: antrian yang sudah habis (semua tiket completed) tetap menerima join baru.
// Sebelumnya sort_key baru bentrok dengan tiket completed (UNIQUE queue_id, sort_key) sehingga 503.
func TestJoinQueue_AfterQueueDrainedStillAcceptsJoin(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "drain-o@b.c", time.Now().Add(time.Hour))
	first := seedUser(t, db, "drain-1@b.c")
	second := seedUser(t, db, "drain-2@b.c")
	q := createQueue(t, db, owner, "Drain")

	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		database.NewSqlxTxRunner(db),
	)
	svc := service.NewTicketService(repository.NewTicketRepository(db), database.NewSqlxTxRunner(db)).WithQueueGuard(queueSvc)

	_, err := svc.JoinQueue(ctx, q.ID, first)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)

	tk, err := svc.JoinQueue(ctx, q.ID, second)
	require.NoError(t, err)
	assert.Equal(t, "waiting", tk.Status)
}

func TestSkipTicket_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	u1, u2, u3 := seedUser(t, db, "1@b.c"), seedUser(t, db, "2@b.c"), seedUser(t, db, "3@b.c")
	q := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		txRunner,
	).WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)

	for _, u := range []int64{u1, u2, u3} {
		_, err := svc.JoinQueue(ctx, q.ID, u)
		require.NoError(t, err)
	}

	_, err := svc.SkipTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrNothingToSkip)

	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)

	called, err := svc.SkipTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	assert.Equal(t, 2, called.TicketNumber)

	st, err := svc.QueueStatus(ctx, q.ID, owner)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)

	skipped, err := svc.ListTickets(ctx, q.ID, owner, []string{"skipped"})
	require.NoError(t, err)
	require.Len(t, skipped, 1)
	assert.Equal(t, 1, skipped[0].TicketNumber)
	assert.Equal(t, 1, skipped[0].SkipCount)
	assert.False(t, skipped[0].CompletedAt.Valid)

	// Skip beruntun: 2 dilewati, 3 dipanggil; lalu tidak ada yang menunggu -> QUEUE_EMPTY, 3 tetap dilayani.
	called, err = svc.SkipTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	assert.Equal(t, 3, called.TicketNumber)
	_, err = svc.SkipTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrQueueEmpty)

	st, err = svc.QueueStatus(ctx, q.ID, owner)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 3, *st.CurrentNumber)
}

func TestSkipFallback_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "fo@b.c", time.Now().Add(time.Hour))
	q := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	queueRepo := repository.NewQueueRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(queueRepo, repository.NewSubscriptionChecker(db), txRunner).
		WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)

	users := []int64{}
	for i := 0; i < 8; i++ {
		users = append(users, seedUser(t, db, fmt.Sprintf("f%d@b.c", i)))
	}
	for _, u := range users {
		_, err := svc.JoinQueue(ctx, q.ID, u)
		require.NoError(t, err)
	}

	_, err := queueSvc.UpdateSkipSettings(ctx, q.ID, owner, model.SkipSettings{Fallback: "after_n", N: 1, MaxReturns: 50})
	require.NoError(t, err)
	got, err := queueSvc.GetSkipSettings(ctx, q.ID, owner)
	require.NoError(t, err)
	assert.Equal(t, 1, got.N)

	// Persist ulang ke Postgres lalu panggil nomor 1 dan skip berulang di titik sisip yang sama.
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)
	for i := 0; i < 25; i++ {
		_, err = svc.SkipTicket(ctx, q.ID, owner)
		require.NoError(t, err, "skip ke-%d", i)
	}

	waiting, err := svc.ListTickets(ctx, q.ID, owner, []string{"waiting"})
	require.NoError(t, err)
	seen := map[int64]bool{}
	for _, w := range waiting {
		assert.False(t, seen[w.SortKey], "sort_key kembar")
		seen[w.SortKey] = true
	}
	assert.Len(t, waiting, 7)

	// back: tiket yang dilewati ke belakang.
	_, err = queueSvc.UpdateSkipSettings(ctx, q.ID, owner, model.SkipSettings{Fallback: "back", MaxReturns: 50})
	require.NoError(t, err)
	served, err := svc.SkipTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	waiting, _ = svc.ListTickets(ctx, q.ID, owner, []string{"waiting"})
	last := waiting[0]
	for _, w := range waiting {
		if w.SortKey > last.SortKey {
			last = w
		}
	}
	assert.NotEqual(t, served.ID, last.ID)
	assert.Equal(t, "waiting", last.Status)
}

func TestRecall_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	u1, u2 := seedUser(t, db, "1@b.c"), seedUser(t, db, "2@b.c")
	q := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		txRunner,
	).WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)

	for _, u := range []int64{u1, u2} {
		_, err := svc.JoinQueue(ctx, q.ID, u)
		require.NoError(t, err)
	}

	_, err := svc.RecallTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrNothingToRecall)

	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)
	_, err = svc.RecallTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrRecallTooSoon)

	_, err = db.ExecContext(ctx, `UPDATE tickets SET called_at = now() - interval '5 seconds' WHERE ticket_number = 1`)
	require.NoError(t, err)
	got, err := svc.RecallTicket(ctx, q.ID, owner)
	require.NoError(t, err)
	assert.Equal(t, 1, got.TicketNumber)

	var recalled bool
	require.NoError(t, db.GetContext(ctx, &recalled, `SELECT recalled_at IS NOT NULL FROM tickets WHERE ticket_number = 1`))
	assert.True(t, recalled)

	_, err = svc.RecallTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrAlreadyRecalled)

	// Nomor berikutnya dipanggil: kesempatan panggil ulang untuk nomor baru kembali (nomor 2 masih di bawah 3 detik).
	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)
	_, err = svc.RecallTicket(ctx, q.ID, owner)
	assert.ErrorIs(t, err, apperror.ErrRecallTooSoon)
}

func TestTicketsNotifyTrigger_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "o@b.c", time.Now().Add(time.Hour))
	u1 := seedUser(t, db, "1@b.c")
	q := createQueue(t, db, owner, "Q")

	ticketRepo := repository.NewTicketRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		txRunner,
	).WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)
	_, err := svc.JoinQueue(ctx, q.ID, u1)
	require.NoError(t, err)

	// Dengar lewat koneksi terpisah di database yang sama (NOTIFY lintas schema, channel global per database).
	listener, err := db.Conn(ctx)
	require.NoError(t, err)
	defer listener.Close()
	_, err = listener.ExecContext(ctx, "LISTEN queue_events")
	require.NoError(t, err)

	_, err = svc.NextTicket(ctx, q.ID, owner, 0)
	require.NoError(t, err)

	var payload string
	err = listener.Raw(func(dc interface{}) error {
		wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		n, err := dc.(*stdlib.Conn).Conn().WaitForNotification(wctx)
		if err != nil {
			return err
		}
		payload = n.Payload
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, payload, `"kind" : "called"`)
}

// Rejoin nyata: setelah tiket selesai, join lagi memberi nomor baru dan tiket lama tetap completed.
// Penutupan antrian menurunkan nomor yang sedang dilayani menjadi completed dalam transaksi yang sama.
func TestRejoin_AndCloseQueue_AgainstPostgres(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	owner := seedOwner(t, db, "rj-o@b.c", time.Now().Add(time.Hour))
	a := seedUser(t, db, "rj-a@b.c")
	b := seedUser(t, db, "rj-b@b.c")
	q := createQueue(t, db, owner, "Rejoin")

	ticketRepo := repository.NewTicketRepository(db)
	txRunner := database.NewSqlxTxRunner(db)
	queueSvc := service.NewQueueService(
		repository.NewQueueRepository(db),
		repository.NewSubscriptionChecker(db),
		txRunner,
	).WithTicketHolderChecker(ticketRepo)
	svc := service.NewTicketService(ticketRepo, txRunner).WithQueueGuard(queueSvc)

	_, err := svc.JoinQueue(ctx, q.ID, a)
	require.NoError(t, err)
	_, err = svc.JoinQueue(ctx, q.ID, b)
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q.ID, owner, 0) // A dilayani
	require.NoError(t, err)
	_, err = svc.NextTicket(ctx, q.ID, owner, 0) // A selesai, B dilayani
	require.NoError(t, err)

	rejoined, err := svc.JoinQueue(ctx, q.ID, a)
	require.NoError(t, err)
	assert.Equal(t, 3, rejoined.TicketNumber)
	assert.Equal(t, "waiting", rejoined.Status)

	// /users/me/tickets: satu baris per antrian, yaitu tiket aktif.
	mine, err := svc.ListMyTickets(ctx, a)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, rejoined.ID, mine[0].ID)

	// Tutup antrian saat B masih dilayani: B ikut menjadi completed, tidak ada yang menggantung.
	require.NoError(t, repository.NewQueueRepository(db).CloseQueue(ctx, q.ID))

	serving, err := ticketRepo.ListByQueue(ctx, q.ID, []string{"currently_served"})
	require.NoError(t, err)
	assert.Empty(t, serving)

	completed, err := ticketRepo.ListByQueue(ctx, q.ID, []string{"completed"})
	require.NoError(t, err)
	require.Len(t, completed, 2)
	for _, tk := range completed {
		assert.True(t, tk.CompletedAt.Valid, "tiket %d harus punya completed_at", tk.TicketNumber)
	}
}
