package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"qms/internal/service"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	specOwnerID   int64 = 1
	specCustomerA int64 = 101
	specCustomerB int64 = 102
	specCustomerC int64 = 103
	specCustomerD int64 = 104
)

// newSpecEnv: antrian aktif milik specOwnerID (US-1: antrian baru langsung kosong dan aktif).
func newSpecEnv(t *testing.T) *specEnv {
	t.Helper()
	env := &specEnv{
		queues:  newMemQueueRepo(),
		tickets: newMemTicketRepo(),
		subs:    &memSubscriptions{active: map[int64]bool{specOwnerID: true}},
		events:  &recordingNotifier{},
		ownerID: specOwnerID,
	}
	env.qs = service.NewQueueService(env.queues, env.subs, fakeTxRunner{}, env.events).
		WithTicketHolderChecker(env.tickets)
	env.ts = service.NewTicketService(env.tickets, fakeTxRunner{}, env.events).
		WithQueueGuard(env.qs)

	q, err := env.qs.CreateQueue(context.Background(), specOwnerID, "Loket Utama", "Antrian harian")
	require.NoError(t, err)
	env.queueID = q.ID
	return env
}

func TestSpec_Join_EmptyQueueGivesFirstNumber(t *testing.T) {
	// TC-05-03 / US-14 AC2: antrian kosong -> nomor pertama (A-001), 0 orang di depan.
	env := newSpecEnv(t)
	ctx := context.Background()

	ticket, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Equal(t, 1, ticket.TicketNumber)
	assert.Equal(t, "waiting", ticket.Status)

	status, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.NotNil(t, status.WaitingAhead)
	assert.Equal(t, 0, *status.WaitingAhead)
}

func TestSpec_Join_AppendsAtEndOfQueue(t *testing.T) {
	// US-14 AC2 / 4.2 langkah 5: pendaftar baru masuk di akhir antrian.
	env := newSpecEnv(t)
	ctx := context.Background()

	var joined []model.Ticket
	for _, c := range []int64{specCustomerA, specCustomerB, specCustomerC} {
		tk, err := env.ts.JoinQueue(ctx, env.queueID, c)
		require.NoError(t, err)
		joined = append(joined, tk)
	}

	assert.Less(t, joined[0].SortKey, joined[1].SortKey)
	assert.Less(t, joined[1].SortKey, joined[2].SortKey)
	assert.Equal(t, []int{1, 2, 3}, []int{joined[0].TicketNumber, joined[1].TicketNumber, joined[2].TicketNumber})
}

func TestSpec_Join_ClosedQueueGivesNoNumberAndNoEvent(t *testing.T) {
	// 4.2 edge case: antrian tertutup -> "queue is currently closed", tidak ada nomor dibuat.
	env := newSpecEnv(t)
	ctx := context.Background()
	require.NoError(t, env.qs.CloseQueue(ctx, env.queueID, specOwnerID))

	_, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
	assert.Empty(t, env.tickets.ticketsOf(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventTicketJoined))
}

func TestSpec_Join_UnknownQueueIsNotFound(t *testing.T) {
	// TC-05-07: antrian tidak ada -> "Antrian tidak ditemukan", tidak ada nomor.
	env := newSpecEnv(t)

	_, err := env.ts.JoinQueue(context.Background(), env.queueID+999, specCustomerA)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, env.tickets.inserts)
}

func TestSpec_Join_PublishesTicketJoinedOnlyOnSuccess(t *testing.T) {
	// NFR 4: server menyiarkan perubahan saat join; join gagal tidak menyiarkan apa pun.
	env := newSpecEnv(t)
	ctx := context.Background()

	_, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.Equal(t, 1, env.events.count(service.EventTicketJoined))

	call := env.events.snapshot()[0]
	assert.Equal(t, env.queueID, call.QueueID)
	ev, ok := call.Payload.(dto.TicketEvent)
	require.True(t, ok, "payload harus dto.TicketEvent")
	assert.Equal(t, 1, ev.TicketNumber)

	_, _ = env.ts.JoinQueue(ctx, env.queueID+999, specCustomerB) // antrian tidak ada
	assert.Equal(t, 1, env.events.count(service.EventTicketJoined))
}

func TestSpec_Join_AlreadyJoinedShowsExistingNumber(t *testing.T) {
	// 4.2 edge case & TC-05-04: scan/join ulang -> tampilkan nomor yang sudah ada,
	// jangan buat nomor kedua. (Dapat gagal: implementasi mengembalikan ErrAlreadyJoined.)
	env := newSpecEnv(t)
	ctx := context.Background()

	first, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)

	again, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err, "join ulang harus menampilkan nomor yang sudah ada, bukan error")
	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, first.TicketNumber, again.TicketNumber)
	assert.Len(t, env.tickets.ticketsOf(env.queueID), 1, "tidak boleh ada tiket kedua")
	assert.Equal(t, 1, env.events.count(service.EventTicketJoined))
}

func TestSpec_Join_DoubleTapCreatesSingleTicket(t *testing.T) {
	// 4.2 / NFR 3: double-tap atau scan berulang tidak boleh membuat dua nomor.
	env := newSpecEnv(t)
	ctx := context.Background()

	const taps = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ids []int64
	var errs []error
	for i := 0; i < taps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tk, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			ids = append(ids, tk.ID)
		}()
	}
	wg.Wait()

	assert.Len(t, env.tickets.ticketsOf(env.queueID), 1, "tepat satu tiket untuk satu user")
	for _, id := range ids {
		assert.Equal(t, ids[0], id, "semua respons sukses harus menunjuk tiket yang sama")
	}
	for _, err := range errs {
		assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)
	}
}

func TestSpec_Next_ServesInJoinOrder(t *testing.T) {
	// TC-03-04 / US-8: nomor dilayani berurutan A-001, A-002, A-003.
	env := newSpecEnv(t)
	ctx := context.Background()
	for _, c := range []int64{specCustomerA, specCustomerB, specCustomerC} {
		_, err := env.ts.JoinQueue(ctx, env.queueID, c)
		require.NoError(t, err)
	}

	var served []int
	for i := 0; i < 3; i++ {
		tk, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
		require.NoError(t, err)
		served = append(served, tk.TicketNumber)
	}
	assert.Equal(t, []int{1, 2, 3}, served)
}

func TestSpec_Next_ServesHeadAndPublishesTicketNext(t *testing.T) {
	// TC-03-02 / 4.3 langkah 3: Next menandai nomor yang dilayani dan menyiarkan ticket_next.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)

	served, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, served.TicketNumber)
	assert.Equal(t, specCustomerA, served.UserID)

	require.Equal(t, 1, env.events.count(service.EventTicketNext))
	ev, ok := env.events.snapshot()[len(env.events.snapshot())-1].Payload.(dto.TicketEvent)
	require.True(t, ok)
	assert.Equal(t, 1, ev.TicketNumber)
	assert.Equal(t, "currently_served", ev.Status)
}

func TestSpec_Next_EmptyQueueReturnsQueueEmptyWithoutStateChange(t *testing.T) {
	// TC-03-01 / 4.3 edge case: Next pada antrian kosong -> "tidak ada yang menunggu", tidak ada perubahan.
	env := newSpecEnv(t)
	ctx := context.Background()

	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrQueueEmpty)
	assert.Equal(t, 0, env.tickets.completes)
	assert.Equal(t, 0, env.events.count(service.EventTicketNext))
}

func TestSpec_Next_AfterLastServedIsQueueEmpty(t *testing.T) {
	// 4.3: tidak ada nomor berikutnya -> sama seperti antrian kosong. Next yang gagal tidak mengubah status:
	// nomor yang sedang dilayani tetap A-001, dan tidak ada tiket yang diturunkan ke completed.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrQueueEmpty)
	assert.Equal(t, 0, env.tickets.completes)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 1, *st.CurrentNumber)
}

func TestSpec_Next_TwiceCompletesFirstAndServesSecond(t *testing.T) {
	// 4.3: Next kedua menurunkan nomor pertama menjadi completed dan menaikkan nomor kedua menjadi currently_served.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	second, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, second.TicketNumber)
	assert.Equal(t, "currently_served", second.Status)
	assert.Equal(t, 1, env.tickets.completes)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, "completed", st.MyTicket.Status)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)
	require.NotNil(t, st.PreviousNumber)
	assert.Equal(t, 1, *st.PreviousNumber)
}

func TestSpec_Next_CustomerIsDeniedWithoutStateChange(t *testing.T) {
	// TC-08-02 / 4.6: customer tidak boleh memanggil NEXT -> 404, tiket tetap menunggu.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.NextTicket(ctx, env.queueID, specCustomerA, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, env.tickets.completes)
	assert.Equal(t, 0, env.events.count(service.EventTicketNext))
}

func TestSpec_Status_CustomerSeesServingNumberAndPeopleAhead(t *testing.T) {
	// TC-06-01 / 4.4: A-003 menunggu, belum ada yang dilayani -> "-" dan 2 orang di depan.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerC)
	require.NoError(t, err)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, 3, st.MyTicket.TicketNumber)
	assert.Nil(t, st.CurrentNumber, "belum ada yang dilayani")
	require.NotNil(t, st.WaitingAhead)
	assert.Equal(t, 2, *st.WaitingAhead)
}

func TestSpec_Status_NextCustomerSeesUpdateWithoutRejoin(t *testing.T) {
	// TC-07-02 / 4.4: setelah NEXT, A-002 melihat A-001 sedang dilayani dan 0 orang di depan.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerB)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 1, *st.CurrentNumber)
	require.NotNil(t, st.WaitingAhead)
	assert.Equal(t, 0, *st.WaitingAhead)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, "waiting", st.MyTicket.Status)
}

func TestSpec_Status_ServedCustomerSeesCurrentlyServed(t *testing.T) {
	// TC-06-02: setelah dipanggil, status tiket "currently_served" dan nomor yang dilayani adalah miliknya.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.NotNil(t, st.MyTicket)
	assert.Equal(t, "currently_served", st.MyTicket.Status)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 1, *st.CurrentNumber)
	assert.Nil(t, st.WaitingAhead, "tiket yang sedang dilayani tidak punya hitungan orang di depan")
}

func TestSpec_Status_HolderSeesClosedQueue(t *testing.T) {
	// TC-07-03: pemegang tiket melihat antrian ditutup tanpa refresh (status antrian "closed").
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, env.qs.CloseQueue(ctx, env.queueID, specOwnerID))

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Equal(t, "closed", st.Queue.Status)
	assert.Equal(t, 1, env.events.count(service.EventQueueClosed))
}

func TestSpec_Status_OutsiderIsNotFoundAndLearnsNothing(t *testing.T) {
	// TC-08-01 / 4.4: pengguna tanpa tiket tidak melihat status (404, tidak bocor).
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerB)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Nil(t, st.MyTicket)
	assert.Nil(t, st.CurrentNumber)
	assert.Nil(t, st.WaitingAhead)
}

func TestSpec_ListTickets_OwnerSeesEveryRegisteredCustomer(t *testing.T) {
	// US-6 / 4.1 langkah 3: owner melihat semua customer yang terdaftar.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)

	tickets, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, nil)
	require.NoError(t, err)
	require.Len(t, tickets, 2)
	assert.Equal(t, specCustomerA, tickets[0].UserID)
	assert.Equal(t, specCustomerB, tickets[1].UserID)
}

func TestSpec_ListTickets_CustomerCannotSeeOthersTickets(t *testing.T) {
	// TC-08-03 / 4.1: customer (bukan owner) tidak boleh melihat daftar customer antrian.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	tickets, err := env.ts.ListTickets(ctx, env.queueID, specCustomerA, nil)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Empty(t, tickets)
}

func TestSpec_Skip_SkipsServingAndCallsNext(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	for _, c := range []int64{specCustomerA, specCustomerB, specCustomerC} {
		_, _ = env.ts.JoinQueue(ctx, env.queueID, c)
	}
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	called, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, called.TicketNumber)
	assert.Equal(t, "currently_served", called.Status)

	all, _ := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"skipped"})
	require.Len(t, all, 1)
	assert.Equal(t, 1, all[0].TicketNumber)
	assert.Equal(t, 1, all[0].SkipCount)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerB)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)
	assert.Equal(t, 1, env.events.count(service.EventTicketSkipped))
	assert.Equal(t, 2, env.events.count(service.EventTicketNext))
}

func TestSpec_Skip_NothingServedIsNothingToSkip(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrNothingToSkip)
	assert.Equal(t, 0, env.events.count(service.EventTicketSkipped))
}

func TestSpec_Skip_NobodyWaitingKeepsServing(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrQueueEmpty)

	// fake tx tidak rollback, jadi cek lewat event: tidak ada yang disiarkan
	assert.Equal(t, 0, env.events.count(service.EventTicketSkipped))
}

func TestSpec_Skip_CustomerIsDenied(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specCustomerA, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func setSkip(env *specEnv, s model.SkipSettings) {
	env.tickets.skipSettings = map[int64]model.SkipSettings{env.queueID: s}
}

func joinN(env *specEnv, n int) {
	ctx := context.Background()
	for _, c := range []int64{specCustomerA, specCustomerB, specCustomerC}[:n] {
		_, _ = env.ts.JoinQueue(ctx, env.queueID, c)
	}
}

func waitingNumbers(t *testing.T, env *specEnv) []int {
	all, err := env.ts.ListTickets(context.Background(), env.queueID, specOwnerID, []string{"waiting"})
	require.NoError(t, err)
	sort.Slice(all, func(i, j int) bool { return all[i].SortKey < all[j].SortKey })
	out := []int{}
	for _, x := range all {
		out = append(out, x.TicketNumber)
	}
	return out
}

func TestSkipFallback_BackRequeuesAtTheEnd(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	setSkip(env, model.SkipSettings{Fallback: model.SkipFallbackBack, MaxReturns: 1})
	joinN(env, 3)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 1}, waitingNumbers(t, env))
}

func TestSkipFallback_AfterNPlacesBehindN(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	setSkip(env, model.SkipSettings{Fallback: model.SkipFallbackAfterN, N: 1, MaxReturns: 1})
	joinN(env, 3)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	// 2 dipanggil; sisa menunggu [3]; tiket 1 mundur 1 -> [3,1]
	assert.Equal(t, []int{3, 1}, waitingNumbers(t, env))
}

func TestSkipFallback_AfterNMoreThanWaitingGoesToBack(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	setSkip(env, model.SkipSettings{Fallback: model.SkipFallbackAfterN, N: 5, MaxReturns: 1})
	joinN(env, 3)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)

	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 1}, waitingNumbers(t, env))
}

func TestSkipFallback_MaxReturnsMakesSkipPermanent(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	setSkip(env, model.SkipSettings{Fallback: model.SkipFallbackBack, MaxReturns: 1})
	joinN(env, 3)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil) // 1 kembali, 2 dilayani
	require.NoError(t, err)
	_, err = env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil) // 2 kembali (skip_count 0), 3 dilayani
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil) // 1 dilayani
	require.NoError(t, err)
	_, err = env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil) // 1 sudah 1x kembali -> permanen
	require.NoError(t, err)

	sk, _ := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"skipped"})
	require.Len(t, sk, 1)
	assert.Equal(t, 1, sk[0].TicketNumber)
}

func serveFirst(t *testing.T, env *specEnv, customers int) {
	joinN(env, customers)
	_, err := env.ts.NextTicket(context.Background(), env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
}

func TestRecall_NothingServed(t *testing.T) {
	env := newSpecEnv(t)
	joinN(env, 1)
	_, err := env.ts.RecallTicket(context.Background(), env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrNothingToRecall)
}

func TestRecall_TooSoonBeforeThreeSeconds(t *testing.T) {
	env := newSpecEnv(t)
	serveFirst(t, env, 2)
	_, err := env.ts.RecallTicket(context.Background(), env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrRecallTooSoon)
	assert.Equal(t, 0, env.tickets.recalls)
}

func TestRecall_AllowedOnceAfterThreeSeconds(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	serveFirst(t, env, 2)
	env.tickets.backdateCall(1, service.RecallDelay+time.Second)

	got, err := env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, got.TicketNumber)
	assert.Equal(t, 1, env.tickets.recalls)

	_, err = env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrAlreadyRecalled)
	assert.Equal(t, 1, env.tickets.recalls)
}

func TestRecall_ResetsAfterNextCall(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	serveFirst(t, env, 2)
	env.tickets.backdateCall(1, service.RecallDelay+time.Second)
	_, err := env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil) // nomor 2 dipanggil
	require.NoError(t, err)
	env.tickets.backdateCall(2, service.RecallDelay+time.Second)
	_, err = env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.NoError(t, err)
}

func TestRecall_CustomerIsDenied(t *testing.T) {
	env := newSpecEnv(t)
	serveFirst(t, env, 2)
	_, err := env.ts.RecallTicket(context.Background(), env.queueID, specCustomerA, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestSpec_Rejoin_AfterCompletedGetsNewNumberAndOldStaysCompleted(t *testing.T) {
	// 4.2 / bagian 3.2: setelah tiket selesai, join lagi ke antrian yang sama memberi nomor baru.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	rejoined, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Equal(t, 3, rejoined.TicketNumber)
	assert.Equal(t, "waiting", rejoined.Status)

	old := env.tickets.ticketsOf(env.queueID)
	var oldA model.Ticket
	for _, tk := range old {
		if tk.TicketNumber == 1 {
			oldA = tk
		}
	}
	assert.Equal(t, "completed", oldA.Status, "tiket lama tetap completed")
}

func TestSpec_Rejoin_WhileWaitingReturnsSameTicket(t *testing.T) {
	// Join saat tiket masih aktif tidak membuat nomor kedua.
	env := newSpecEnv(t)
	ctx := context.Background()
	first, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	again, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID)
	assert.Len(t, env.tickets.ticketsOf(env.queueID), 1)
}

func TestSpec_ListMyTickets_OneRowPerQueue(t *testing.T) {
	// /users/me/tickets: satu baris per antrian walau ada riwayat lama di antrian yang sama.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	mine, err := env.ts.ListMyTickets(ctx, specCustomerA)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, 3, mine[0].TicketNumber, "baris yang tampil adalah tiket aktif terbaru")
	assert.Equal(t, "waiting", mine[0].Status)
}
