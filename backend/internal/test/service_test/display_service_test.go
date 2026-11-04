package service_test

import (
	"context"
	"database/sql"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDisplayRepo struct {
	mu        sync.Mutex
	displays  map[int64]model.QueueDisplay
	info      model.DisplayQueueInfo
	upcoming  []int64
	skipped   []int64
	builds    atomic.Int32
	gotUpcLim int
	gotSkpLim int
}

func newFakeDisplayRepo() *fakeDisplayRepo {
	return &fakeDisplayRepo{
		displays: map[int64]model.QueueDisplay{},
		info:     model.DisplayQueueInfo{Name: "Loket A", Prefix: "B", Status: "active"},
	}
}

func (r *fakeDisplayRepo) Get(_ context.Context, id int64) (model.QueueDisplay, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.displays[id]
	if !ok {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	return d, nil
}

func (r *fakeDisplayRepo) GetByToken(_ context.Context, token string) (model.QueueDisplay, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builds.Add(1)
	for _, d := range r.displays {
		if d.Token == token {
			return d, nil
		}
	}
	return model.QueueDisplay{}, apperror.ErrNotFound
}

func (r *fakeDisplayRepo) Upsert(_ context.Context, id int64, token string, up, sk int) (model.QueueDisplay, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.displays[id]
	if !ok {
		d = model.QueueDisplay{QueueID: id, Token: token}
	}
	d.UpcomingCount, d.SkippedCount = up, sk
	r.displays[id] = d
	return d, nil
}

func (r *fakeDisplayRepo) Rotate(_ context.Context, id int64, token string) (model.QueueDisplay, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.displays[id]
	if !ok {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	d.Token = token
	r.displays[id] = d
	return d, nil
}

func (r *fakeDisplayRepo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.displays, id)
	return nil
}

func (r *fakeDisplayRepo) QueueInfo(context.Context, int64) (model.DisplayQueueInfo, error) {
	return r.info, nil
}

func (r *fakeDisplayRepo) UpcomingNumbers(_ context.Context, _ int64, limit int) ([]int64, error) {
	r.gotUpcLim = limit
	return r.upcoming, nil
}

func (r *fakeDisplayRepo) SkippedNumbers(_ context.Context, _ int64, limit int) ([]int64, error) {
	r.gotSkpLim = limit
	return r.skipped, nil
}

type fakeDisplayCounters struct{ list []model.CounterWithServing }

func (f *fakeDisplayCounters) ListWithServing(context.Context, int64) ([]model.CounterWithServing, error) {
	return f.list, nil
}

func counterRow(id int64, name string, serving int64) model.CounterWithServing {
	c := model.CounterWithServing{Counter: model.Counter{ID: id, Name: name}}
	if serving > 0 {
		c.ServingNumber = sql.NullInt64{Int64: serving, Valid: true}
		c.ServingTicketID = sql.NullInt64{Int64: serving * 100, Valid: true}
	}
	return c
}

func newDisplaySvc() (*service.DisplayService, *fakeDisplayRepo, *fakeDisplayCounters, *recordingNotifier) {
	repo := newFakeDisplayRepo()
	counters := &fakeDisplayCounters{list: []model.CounterWithServing{counterRow(1, "Counter 1", 0)}}
	n := &recordingNotifier{}
	svc := service.NewDisplayService(repo, counters, nil).
		WithQueueGuard(fakeQueueGuard{ownerID: 7}).
		WithNotifier(n)
	return svc, repo, counters, n
}

func TestDisplay_OnlyOwnerAndRangeValidated(t *testing.T) {
	svc, repo, _, _ := newDisplaySvc()
	ctx := context.Background()

	_, err := svc.Configure(ctx, 5, 99, 5, 0)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "bukan owner")
	for _, c := range [][2]int{{-1, 0}, {11, 0}, {5, -1}, {5, 11}} {
		_, err = svc.Configure(ctx, 5, 7, c[0], c[1])
		assert.ErrorIs(t, err, apperror.ErrInvalidDisplayConfig, "%v", c)
	}
	assert.Empty(t, repo.displays, "tidak ada yang tersimpan")

	d, err := svc.Configure(ctx, 5, 7, 0, 10)
	require.NoError(t, err)
	assert.Len(t, d.Token, 32, "24 byte base64url")

	_, err = svc.Config(ctx, 5, 99)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.ErrorIs(t, svc.Disable(ctx, 5, 99), apperror.ErrNotFound)
	_, err = svc.Rotate(ctx, 5, 99)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestDisplay_ConfigureKeepsTokenAndRotateChangesIt(t *testing.T) {
	svc, _, _, notifier := newDisplaySvc()
	ctx := context.Background()
	first, err := svc.Configure(ctx, 5, 7, 5, 0)
	require.NoError(t, err)
	again, err := svc.Configure(ctx, 5, 7, 3, 2)
	require.NoError(t, err)
	assert.Equal(t, first.Token, again.Token, "mengubah jumlah tidak mengganti token")

	rotated, err := svc.Rotate(ctx, 5, 7)
	require.NoError(t, err)
	assert.NotEqual(t, first.Token, rotated.Token)

	_, err = svc.Snapshot(ctx, first.Token)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "token lama berhenti berlaku")
	_, err = svc.Snapshot(ctx, rotated.Token)
	assert.NoError(t, err)

	require.NoError(t, svc.Disable(ctx, 5, 7))
	require.NoError(t, svc.Disable(ctx, 5, 7), "idempoten")
	_, err = svc.Snapshot(ctx, rotated.Token)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.GreaterOrEqual(t, len(notifier.snapshot()), 4, "layar yang terbuka diberi sinyal tiap perubahan")
}

func TestDisplay_SnapshotShapeSingleAndMultiCounter(t *testing.T) {
	svc, repo, counters, _ := newDisplaySvc()
	ctx := context.Background()
	d, err := svc.Configure(ctx, 5, 7, 2, 1)
	require.NoError(t, err)
	repo.upcoming, repo.skipped = []int64{8, 9}, []int64{3}
	counters.list = []model.CounterWithServing{counterRow(1, "Counter 1", 7)}

	snap, err := svc.Snapshot(ctx, d.Token)
	require.NoError(t, err)
	assert.False(t, snap.MultiCounter, "satu counter: nama counter tidak ditampilkan")
	assert.Equal(t, []model.DisplayCounter{{ID: 1, Name: "Counter 1", Serving: "B-007"}}, snap.Counters)
	assert.Equal(t, []string{"B-008", "B-009"}, snap.Upcoming)
	assert.Equal(t, []string{"B-003"}, snap.Skipped)
	assert.Equal(t, 2, repo.gotUpcLim)
	assert.Equal(t, 1, repo.gotSkpLim)

	svc2, repo2, counters2, _ := newDisplaySvc()
	d2, _ := svc2.Configure(ctx, 5, 7, 0, 0)
	counters2.list = []model.CounterWithServing{counterRow(1, "Counter 1", 7), counterRow(2, "Loket Cepat", 0)}
	snap2, err := svc2.Snapshot(ctx, d2.Token)
	require.NoError(t, err)
	assert.True(t, snap2.MultiCounter)
	require.Len(t, snap2.Counters, 2, "counter kosong tetap tampil sebagai kartu")
	assert.Equal(t, "", snap2.Counters[1].Serving)
	assert.Empty(t, snap2.Upcoming, "jumlah 0 = kosong")
	assert.Equal(t, 0, repo2.gotUpcLim)
}

func TestDisplay_EmptyPrefixFallsBackToA(t *testing.T) {
	svc, repo, counters, _ := newDisplaySvc()
	repo.info.Prefix = ""
	counters.list = []model.CounterWithServing{counterRow(1, "x", 42)}
	d, _ := svc.Configure(context.Background(), 5, 7, 1, 0)
	snap, err := svc.Snapshot(context.Background(), d.Token)
	require.NoError(t, err)
	assert.Equal(t, "A-042", snap.Counters[0].Serving)
}

func TestDisplay_MalformedTokenRejectedWithoutDatabase(t *testing.T) {
	svc, repo, _, _ := newDisplaySvc()
	for _, tok := range []string{"", "x", "short", "has space and is exactly 32 chars!!", "../../etc/passwd/../../etc/pass", "ÄÖÜ"} {
		_, err := svc.Snapshot(context.Background(), tok)
		assert.ErrorIs(t, err, apperror.ErrNotFound, tok)
	}
	_, err := svc.QueueIDForToken(context.Background(), "x")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Zero(t, repo.builds.Load(), "token berbentuk salah tidak sampai ke database")
}

func TestDisplayCache_ReusesSnapshotWithinTTLAndInvalidateForcesReload(t *testing.T) {
	svc, repo, counters, _ := newDisplaySvc()
	ctx := context.Background()
	d, _ := svc.Configure(ctx, 5, 7, 1, 0)
	counters.list = []model.CounterWithServing{counterRow(1, "c", 1)}

	for i := 0; i < 5; i++ {
		_, err := svc.Snapshot(ctx, d.Token)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), repo.builds.Load(), "lima permintaan, satu pembangunan snapshot")
}

func TestDisplayCache_InvalidateBeforeEventGivesFreshData(t *testing.T) {
	cache := service.NewDisplayCache(time.Minute)
	repo := newFakeDisplayRepo()
	counters := &fakeDisplayCounters{list: []model.CounterWithServing{counterRow(1, "c", 1)}}
	svc := service.NewDisplayService(repo, counters, cache).WithQueueGuard(fakeQueueGuard{ownerID: 7})
	ctx := context.Background()
	d, _ := svc.Configure(ctx, 5, 7, 1, 0)

	first, _ := svc.Snapshot(ctx, d.Token)
	assert.Equal(t, "B-001", first.Counters[0].Serving)

	counters.list = []model.CounterWithServing{counterRow(1, "c", 2)}
	stale, _ := svc.Snapshot(ctx, d.Token)
	assert.Equal(t, "B-001", stale.Counters[0].Serving, "TTL panjang: masih cache")

	cache.Invalidate(5) // yang dilakukan Hub sebelum menyebar event
	fresh, _ := svc.Snapshot(ctx, d.Token)
	assert.Equal(t, "B-002", fresh.Counters[0].Serving)
}

func TestDisplayCache_ConcurrentRequestsShareOneLoad(t *testing.T) {
	cache := service.NewDisplayCache(time.Minute)
	var loads atomic.Int32
	release := make(chan struct{})
	load := func() (int64, model.DisplaySnapshot, error) {
		loads.Add(1)
		<-release
		return 5, model.DisplaySnapshot{QueueName: "q"}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, snap, err := cache.Get("tok", load)
			assert.NoError(t, err)
			assert.Equal(t, "q", snap.QueueName)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	assert.Equal(t, int32(1), loads.Load())
}

func TestDisplayCache_LoadStartedBeforeInvalidateIsNotStored(t *testing.T) {
	cache := service.NewDisplayCache(time.Minute)
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = cache.Get("tok", func() (int64, model.DisplaySnapshot, error) {
			close(started)
			<-release
			return 5, model.DisplaySnapshot{QueueName: "lama"}, nil
		})
	}()
	<-started
	cache.Invalidate(5)
	close(release)
	<-done

	_, snap, _ := cache.Get("tok", func() (int64, model.DisplaySnapshot, error) {
		return 5, model.DisplaySnapshot{QueueName: "baru"}, nil
	})
	assert.Equal(t, "baru", snap.QueueName, "hasil pemuatan yang basi tidak boleh tersimpan")
}
