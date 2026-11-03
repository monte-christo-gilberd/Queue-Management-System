package repository_test

import (
	"context"
	"database/sql"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/repository"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake in-memory untuk antarmuka repository (repo_interface.go).
// Fake ini menggantikan database pada unit test: aturan yang ditiru dari spesifikasi
// (product-overview.md) dicatat di setiap method. Kode SQL nyata tidak diuji di sini.

var (
	_ repository.QueueRepositoryInterface     = (*memQueues)(nil)
	_ repository.SubscriptionCheckerInterface = (*memSubs)(nil)
	_ repository.TicketRepositoryInterface    = (*memTickets)(nil)
	_ repository.UserRepositoryInterface      = (*memUsers)(nil)
)

// memQueues: satu baris per antrian. Antrian baru selalu "active"; CloseQueue -> "closed" dan set closed_at.
type memQueues struct {
	mu       sync.Mutex
	nextID   int64
	rows     map[int64]model.Queue
	order    []int64
	counters []model.Counter
}

func newMemQueues() *memQueues {
	return &memQueues{rows: map[int64]model.Queue{}}
}

func (m *memQueues) CreateNewQueue(_ context.Context, _ *sqlx.Tx, ownerID int64, name, desc string) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	q := model.Queue{ID: m.nextID, OwnerID: ownerID, QueueName: name,
		QueueDesc: sql.NullString{String: desc, Valid: desc != ""}, Status: "active", CreatedAt: time.Now()}
	m.rows[q.ID] = q
	m.order = append(m.order, q.ID)
	return q, nil
}

func (m *memQueues) GetQueueByID(_ context.Context, id int64) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.rows[id]
	if !ok {
		return model.Queue{}, apperror.ErrNotFound
	}
	return q, nil
}

func (m *memQueues) ListQueueByOwner(_ context.Context, ownerID int64, activeOnly bool) ([]model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Queue{}
	for _, id := range m.order {
		q := m.rows[id]
		if q.OwnerID == ownerID && (!activeOnly || q.Status == "active") {
			out = append(out, q)
		}
	}
	return out, nil
}

func (m *memQueues) SetTicketPrefix(_ context.Context, queueID int64, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.rows[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	q.TicketPrefix = prefix
	m.rows[queueID] = q
	return nil
}

func (m *memQueues) UpdateQueueDetails(_ context.Context, id int64, name, desc string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.rows[id]
	if !ok {
		return apperror.ErrNotFound
	}
	q.QueueName = name
	q.QueueDesc = sql.NullString{String: desc, Valid: desc != ""}
	m.rows[id] = q
	return nil
}

func (m *memQueues) CloseQueue(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.rows[id]
	if !ok {
		return apperror.ErrNotFound
	}
	q.Status = "closed"
	q.ClosedAt = sql.NullTime{Time: time.Now(), Valid: true}
	m.rows[id] = q
	return nil
}

func (m *memQueues) GetQueueSettings(_ context.Context, id int64) (model.QueueSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.rows[id]
	if !ok {
		return model.QueueSettings{}, apperror.ErrNotFound
	}
	return model.QueueSettings{QueueID: q.ID, QRType: "static"}, nil
}

func (m *memQueues) CreateQueueSettingsIfMissing(_ context.Context, id int64) error {
	return nil
}

func (m *memQueues) GetSkipSettings(_ context.Context, id int64) (model.SkipSettings, error) {
	return model.DefaultSkipSettings(), nil
}

func (m *memQueues) UpdateSkipSettings(_ context.Context, id int64, s model.SkipSettings) error {
	return nil
}

func (m *memQueues) UpdateQRType(_ context.Context, id int64, qrType string) error {
	return nil
}

func (m *memQueues) UpdateQRCurrentCode(_ context.Context, id int64, token string) error {
	return nil
}

func (m *memQueues) CreateDefaultCounter(
	_ context.Context,
	queueID int64,
	name string,
) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	counter := model.Counter{ID: m.nextID + 1, QueueID: queueID, Name: name, IsDefault: true}
	m.counters = append(m.counters, counter)
	return counter, nil
}

func (m *memQueues) GetCounterByQueue(_ context.Context, _ int64) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.counters) == 0 {
		return model.Counter{}, apperror.ErrNotFound
	}
	return m.counters[0], nil
}

// memSubs: langganan owner. Aktif bila expiry masa depan (data model 4.18).
type memSubs struct {
	expiry map[int64]time.Time
}

func (m *memSubs) IsActive(_ context.Context, ownerID int64) (bool, error) {
	exp, ok := m.expiry[ownerID]
	return ok && exp.After(time.Now()), nil
}

// memTickets: tabel tickets. UNIQUE (queue_id, user_id) hanya untuk tiket aktif -> ErrDuplicate.
type memTickets struct {
	mu      sync.Mutex
	nextID  int64
	numbers map[int64]int
	rows    []*model.Ticket
}

func newMemTickets() *memTickets {
	return &memTickets{numbers: map[int64]int{}}
}

func (m *memTickets) MaxWaitingSortKey(_ context.Context, _ *sqlx.Tx, queueID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var max int64
	for _, t := range m.rows {
		if t.QueueID == queueID && t.Status == "waiting" && t.SortKey > max {
			max = t.SortKey
		}
	}
	return max, nil
}

func (m *memTickets) Insert(_ context.Context, _ *sqlx.Tx, queueID, userID, sortKey int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.rows {
		if t.QueueID == queueID && t.UserID == userID && (t.Status == "waiting" || t.Status == "currently_served") {
			return model.Ticket{}, apperror.ErrDuplicate
		}
	}
	m.nextID++
	m.numbers[queueID]++
	t := &model.Ticket{ID: m.nextID, QueueID: queueID, TicketNumber: m.numbers[queueID],
		SortKey: sortKey, UserID: userID, Status: "waiting", CreatedAt: time.Now()}
	m.rows = append(m.rows, t)
	return *t, nil
}

func (m *memTickets) NextWaiting(_ context.Context, _ *sqlx.Tx, queueID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best *model.Ticket
	for _, t := range m.rows {
		if t.QueueID == queueID && t.Status == "waiting" && (best == nil || t.SortKey < best.SortKey) {
			best = t
		}
	}
	if best == nil {
		return model.Ticket{}, apperror.ErrNotFound
	}
	return *best, nil
}

func (m *memTickets) MarkCompleted(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.rows {
		if t.ID == ticketID {
			t.Status = "completed"
			t.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memTickets) MarkServing(_ context.Context, _ *sqlx.Tx, ticketID int64, counterID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.rows {
		if t.ID == ticketID {
			t.Status = "currently_served"
			t.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
			t.CalledCounterID = counterID
			t.RecalledAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memTickets) CurrentServing(_ context.Context, _ *sqlx.Tx, queueID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.rows {
		if t.QueueID == queueID && t.Status == "currently_served" {
			return *t, nil
		}
	}
	return model.Ticket{}, apperror.ErrNotFound
}

func (m *memTickets) MarkRecalled(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	return nil
}

func (m *memTickets) MarkSkipped(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.rows {
		if t.ID == ticketID {
			t.Status = "skipped"
			t.SkipCount++
			t.CompletedAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

// memUsers: tabel users. UNIQUE(email) -> ErrDuplicate.
type memUsers struct {
	mu     sync.Mutex
	nextID int64
	rows   []model.User
}

func (m *memUsers) Create(_ context.Context, email, username, hash string) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.rows {
		if u.Email == email {
			return model.User{}, apperror.ErrDuplicate
		}
	}
	m.nextID++
	u := model.User{ID: m.nextID, Email: email, Username: username,
		PasswordHash: sql.NullString{String: hash, Valid: true}, CreatedAt: time.Now()}
	m.rows = append(m.rows, u)
	return u, nil
}

func (m *memUsers) FindByEmail(_ context.Context, email string) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.rows {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

func TestRepoFake_QueueLifecycleCreateCloseAndListFilter(t *testing.T) {
	// 4.1 / 4.15: antrian baru aktif; setelah ditutup status closed dan closed_at terisi;
	// daftar aktif tidak memuat antrian yang sudah ditutup.
	ctx := context.Background()
	repo := newMemQueues()

	q1, err := repo.CreateNewQueue(ctx, nil, 7, "Loket 1", "")
	require.NoError(t, err)
	q2, err := repo.CreateNewQueue(ctx, nil, 7, "Loket 2", "")
	require.NoError(t, err)
	assert.Equal(t, "active", q1.Status)
	assert.False(t, q1.QueueDesc.Valid, "deskripsi kosong disimpan sebagai NULL")

	require.NoError(t, repo.CloseQueue(ctx, q1.ID))
	closed, err := repo.GetQueueByID(ctx, q1.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", closed.Status)
	assert.True(t, closed.ClosedAt.Valid)

	active, err := repo.ListQueueByOwner(ctx, 7, true)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, q2.ID, active[0].ID)

	all, err := repo.ListQueueByOwner(ctx, 7, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestRepoFake_QueueMissingRowsAreNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newMemQueues()

	_, err := repo.GetQueueByID(ctx, 404)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.ErrorIs(t, repo.CloseQueue(ctx, 404), apperror.ErrNotFound)
	assert.ErrorIs(t, repo.UpdateQueueDetails(ctx, 404, "x", ""), apperror.ErrNotFound)
}

func TestRepoFake_SubscriptionActiveOnlyBeforeExpiry(t *testing.T) {
	// 4.18 / data model: aktif = subscription_expires_at di masa depan.
	ctx := context.Background()
	subs := &memSubs{expiry: map[int64]time.Time{
		1: time.Now().Add(24 * time.Hour),
		2: time.Now().Add(-time.Hour),
	}}

	active, err := subs.IsActive(ctx, 1)
	require.NoError(t, err)
	assert.True(t, active)

	expired, err := subs.IsActive(ctx, 2)
	require.NoError(t, err)
	assert.False(t, expired)

	never, err := subs.IsActive(ctx, 3)
	require.NoError(t, err)
	assert.False(t, never, "owner yang belum pernah berlangganan tidak aktif")
}

func TestRepoFake_TicketOnePerUserPerQueue(t *testing.T) {
	// 4.2 / NFR 3: satu tiket per (antrian, user); antrian lain boleh.
	ctx := context.Background()
	repo := newMemTickets()

	_, err := repo.Insert(ctx, nil, 1, 42, 65536)
	require.NoError(t, err)

	_, err = repo.Insert(ctx, nil, 1, 42, 131072)
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	_, err = repo.Insert(ctx, nil, 2, 42, 65536)
	assert.NoError(t, err, "user yang sama boleh di antrian lain")
}

func TestRepoFake_TicketConcurrentDuplicateInsertOnlyOneWins(t *testing.T) {
	// NFR 3: double-tap pada waktu yang sama tidak menghasilkan dua tiket.
	ctx := context.Background()
	repo := newMemTickets()

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.Insert(ctx, nil, 1, 42, 65536); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, wins)
}

func TestRepoFake_NextWaitingFollowsSortKeyOrder(t *testing.T) {
	// TC-03-04 / US-14: urutan dilayani = urutan bergabung (sort_key kecil lebih dulu).
	ctx := context.Background()
	repo := newMemTickets()
	maxKey, err := repo.MaxWaitingSortKey(ctx, nil, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), maxKey, "antrian kosong")

	var ids []int64
	for user := int64(101); user <= 103; user++ {
		maxKey, _ = repo.MaxWaitingSortKey(ctx, nil, 1)
		tk, err := repo.Insert(ctx, nil, 1, user, maxKey+65536)
		require.NoError(t, err)
		ids = append(ids, tk.ID)
	}

	for _, want := range ids {
		next, err := repo.NextWaiting(ctx, nil, 1)
		require.NoError(t, err)
		assert.Equal(t, want, next.ID)
		require.NoError(t, repo.MarkCompleted(ctx, nil, next.ID))
	}

	_, err = repo.NextWaiting(ctx, nil, 1)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "tidak ada yang menunggu")
}

func TestRepoFake_MarkCompletedUnknownTicketIsNotFound(t *testing.T) {
	assert.ErrorIs(t, newMemTickets().MarkCompleted(context.Background(), nil, 999), apperror.ErrNotFound)
}

func TestRepoFake_UserEmailIsUnique(t *testing.T) {
	// TC-01-07: email sudah terdaftar -> ErrDuplicate; pencarian email yang tidak ada -> ErrNotFound.
	ctx := context.Background()
	repo := &memUsers{}

	_, err := repo.Create(ctx, "a@example.com", "a", "hash")
	require.NoError(t, err)
	_, err = repo.Create(ctx, "a@example.com", "b", "hash2")
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	u, err := repo.FindByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, "a", u.Username)

	_, err = repo.FindByEmail(ctx, "tidak@ada.com")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}
