package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"strings"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memCounterRepo: fake service.CounterRepository. Satu antrian, counter default sudah ada.
type memCounterRepo struct {
	mu       sync.Mutex
	queueID  int64
	nextID   int64
	counters []model.Counter
}

func newMemCounterRepo(queueID int64) *memCounterRepo {
	return &memCounterRepo{
		queueID:  queueID,
		nextID:   1,
		counters: []model.Counter{{ID: 1, QueueID: queueID, Name: "Counter 1", IsDefault: true}},
	}
	// nextID diisi setelah counter default, lihat Insert.
}

func (m *memCounterRepo) ListWithServing(_ context.Context, queueID int64) ([]model.CounterWithServing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.CounterWithServing{}
	for _, c := range m.counters {
		if c.QueueID == queueID {
			out = append(out, model.CounterWithServing{Counter: c})
		}
	}
	return out, nil
}

func (m *memCounterRepo) LockQueue(_ context.Context, _ *sqlx.Tx, _ int64) error { return nil }

func (m *memCounterRepo) Names(_ context.Context, _ *sqlx.Tx, queueID int64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := []string{}
	for _, c := range m.counters {
		if c.QueueID == queueID {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

func (m *memCounterRepo) Insert(_ context.Context, _ *sqlx.Tx, queueID int64, name string) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.counters {
		if c.QueueID == queueID && c.Name == name {
			return model.Counter{}, apperror.ErrDuplicateCounterName
		}
	}
	m.nextID++
	c := model.Counter{ID: m.nextID, QueueID: queueID, Name: name}
	m.counters = append(m.counters, c)
	return c, nil
}

func (m *memCounterRepo) Get(_ context.Context, _ *sqlx.Tx, queueID, counterID int64) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.counters {
		if c.QueueID == queueID && c.ID == counterID {
			return c, nil
		}
	}
	return model.Counter{}, apperror.ErrNotFound
}

func (m *memCounterRepo) Rename(_ context.Context, _ *sqlx.Tx, queueID, counterID int64, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.counters {
		if c.QueueID == queueID && c.Name == name && c.ID != counterID {
			return apperror.ErrDuplicateCounterName
		}
	}
	for i := range m.counters {
		if m.counters[i].QueueID == queueID && m.counters[i].ID == counterID {
			m.counters[i].Name = name
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memCounterRepo) Delete(_ context.Context, _ *sqlx.Tx, queueID, counterID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, c := range m.counters {
		if c.QueueID == queueID && c.ID == counterID {
			m.counters = append(m.counters[:i], m.counters[i+1:]...)
			return nil
		}
	}
	return apperror.ErrNotFound
}

// newCounterSvc: CounterService di atas antrian dari newSpecEnv. Pelepasan nomor memakai TicketService yang sama.
func newCounterSvc(env *specEnv) (*service.CounterService, *memCounterRepo) {
	repo := newMemCounterRepo(env.queueID)
	svc := service.NewCounterService(repo, fakeTxRunner{}, env.events).
		WithQueueGuard(env.qs).
		WithTicketReleaser(env.ts)
	return svc, repo
}

func counterNames(repo *memCounterRepo) []string {
	names := []string{}
	for _, c := range repo.counters {
		names = append(names, c.Name)
	}
	return names
}

func TestCounter_CreateWithoutNameGetsNextAutoName(t *testing.T) {
	// Tab baru: nama otomatis "Counter 2", lalu "Counter 3".
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)
	ctx := context.Background()

	first, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	require.NoError(t, err)
	second, err := svc.Create(ctx, env.queueID, specOwnerID, "   ")
	require.NoError(t, err)

	assert.Equal(t, "Counter 2", first.Name)
	assert.Equal(t, "Counter 3", second.Name)
	assert.False(t, first.IsDefault)
	assert.Equal(t, []string{"Counter 1", "Counter 2", "Counter 3"}, counterNames(repo))
	assert.Equal(t, 2, env.events.count(service.EventCounterUpdated))
}

func TestCounter_CreateWithNameTrimsIt(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)

	got, err := svc.Create(context.Background(), env.queueID, specOwnerID, "  Loket Dua  ")

	require.NoError(t, err)
	assert.Equal(t, "Loket Dua", got.Name)
}

func TestCounter_DuplicateNameIsRejected(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()
	_, err := svc.Create(ctx, env.queueID, specOwnerID, "Loket Dua")
	require.NoError(t, err)

	_, err = svc.Create(ctx, env.queueID, specOwnerID, "Loket Dua")
	assert.ErrorIs(t, err, apperror.ErrDuplicateCounterName)

	_, err = svc.Create(ctx, env.queueID, specOwnerID, "Counter 1")
	assert.ErrorIs(t, err, apperror.ErrDuplicateCounterName, "nama counter default juga tidak boleh dipakai ulang")
}

func TestCounter_NameLengthIsValidated(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()

	_, err := svc.Create(ctx, env.queueID, specOwnerID, strings.Repeat("a", service.MaxCounterNameLength+1))
	assert.ErrorIs(t, err, apperror.ErrInvalidCounterName)

	got, err := svc.Create(ctx, env.queueID, specOwnerID, strings.Repeat("é", service.MaxCounterNameLength))
	require.NoError(t, err, "batas dihitung dalam karakter, bukan byte")
	assert.Equal(t, service.MaxCounterNameLength, len([]rune(got.Name)))

	_, err = svc.Rename(ctx, env.queueID, specOwnerID, got.ID, "   ")
	assert.ErrorIs(t, err, apperror.ErrInvalidCounterName, "ganti nama tidak boleh kosong")
}

func TestCounter_LimitOfTenCounters(t *testing.T) {
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)
	ctx := context.Background()
	// Counter default sudah 1; tambah 9 lagi sampai batas 10.
	for i := 0; i < service.MaxCounters-1; i++ {
		_, err := svc.Create(ctx, env.queueID, specOwnerID, "")
		require.NoError(t, err)
	}
	require.Len(t, repo.counters, service.MaxCounters)

	_, err := svc.Create(ctx, env.queueID, specOwnerID, "")

	assert.ErrorIs(t, err, apperror.ErrCounterLimit)
	assert.Len(t, repo.counters, service.MaxCounters, "tidak ada counter yang terlampaui")
}

func TestCounter_NonOwnerIsNotFound(t *testing.T) {
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)
	ctx := context.Background()

	_, err := svc.Create(ctx, env.queueID, specCustomerA, "")
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	_, err = svc.Rename(ctx, env.queueID, specCustomerA, 1, "Curi")
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	err = svc.Delete(ctx, env.queueID, specCustomerA, 1)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	assert.Equal(t, []string{"Counter 1"}, counterNames(repo), "tidak ada perubahan oleh non-owner")
}

func TestCounter_ClosedQueueRejectsEverything(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()
	require.NoError(t, env.queues.CloseQueue(ctx, env.queueID))

	_, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)

	_, err = svc.Rename(ctx, env.queueID, specOwnerID, 1, "Baru")
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)

	err = svc.Delete(ctx, env.queueID, specOwnerID, 1)
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
}

func TestCounter_RenameDefaultIsAllowed(t *testing.T) {
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)

	got, err := svc.Rename(context.Background(), env.queueID, specOwnerID, 1, "Meja Utama")

	require.NoError(t, err)
	assert.Equal(t, "Meja Utama", got.Name)
	assert.Equal(t, []string{"Meja Utama"}, counterNames(repo))
}

func TestCounter_RenameToDuplicateOrUnknownIsRejected(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()
	two, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	require.NoError(t, err)

	_, err = svc.Rename(ctx, env.queueID, specOwnerID, two.ID, "Counter 1")
	assert.ErrorIs(t, err, apperror.ErrDuplicateCounterName)

	_, err = svc.Rename(ctx, env.queueID, specOwnerID, 999, "Hantu")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestCounter_DefaultCounterCannotBeDeleted(t *testing.T) {
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)

	err := svc.Delete(context.Background(), env.queueID, specOwnerID, 1)

	assert.ErrorIs(t, err, apperror.ErrDefaultCounter)
	assert.Equal(t, []string{"Counter 1"}, counterNames(repo))
}

func TestCounter_DeleteIdleCounterRemovesIt(t *testing.T) {
	env := newSpecEnv(t)
	svc, repo := newCounterSvc(env)
	ctx := context.Background()
	two, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, env.queueID, specOwnerID, two.ID))

	assert.Equal(t, []string{"Counter 1"}, counterNames(repo))
	assert.Equal(t, 2, env.events.count(service.EventCounterUpdated), "buat dan hapus memancarkan event")
}

func TestCounter_DeleteReturnsServingNumberToFrontWithoutSkipCount(t *testing.T) {
	// D2: menghapus counter yang sedang melayani mengembalikan nomornya ke posisi 1, skip_count tidak naik.
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()
	a, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	two, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	require.NoError(t, err)
	require.NoError(t, env.tickets.MarkServing(ctx, nil, a.ID, two.ID))

	require.NoError(t, svc.Delete(ctx, env.queueID, specOwnerID, two.ID))

	order := env.tickets.waitingInOrder(env.queueID)
	require.Len(t, order, 2)
	assert.Equal(t, a.ID, order[0].ID, "nomor yang dilepas kembali paling depan")
	assert.Equal(t, 0, order[0].SkipCount, "skip_count tidak naik")
	assert.Equal(t, int64(0), order[0].CalledCounterID)
}

func TestCounter_DeleteWithoutReleaserFailsClosed(t *testing.T) {
	env := newSpecEnv(t)
	svc := service.NewCounterService(newMemCounterRepo(env.queueID), fakeTxRunner{}, env.events).
		WithQueueGuard(env.qs)
	_, err := svc.Create(context.Background(), env.queueID, specOwnerID, "")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), env.queueID, specOwnerID, 2)

	assert.Error(t, err)
}

func TestCounter_ListReturnsCountersDefaultFirst(t *testing.T) {
	env := newSpecEnv(t)
	svc, _ := newCounterSvc(env)
	ctx := context.Background()
	_, err := svc.Create(ctx, env.queueID, specOwnerID, "")
	require.NoError(t, err)

	got, err := svc.List(ctx, env.queueID, specOwnerID)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.True(t, got[0].IsDefault)
	assert.Equal(t, "Counter 2", got[1].Name)
}
