package service_test

import (
	"context"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHistoryRepo meniru keyset repository: urut closed_at menurun lalu id menurun, dan mencatat argumen terakhir.
type fakeHistoryRepo struct {
	queues  []model.HistoryQueue
	tickets []model.HistoryTicket
	owner   int64

	gotAfter     *model.HistoryQueueCursor
	gotLimit     int
	gotTicketCut int
}

func (f *fakeHistoryRepo) ListClosedQueues(_ context.Context, ownerID int64, after *model.HistoryQueueCursor, limit int) ([]model.HistoryQueue, error) {
	f.gotAfter, f.gotLimit = after, limit
	if ownerID != f.owner {
		return []model.HistoryQueue{}, nil
	}
	out := []model.HistoryQueue{}
	for _, q := range f.queues {
		if after != nil && !(q.ClosedAt.Before(after.ClosedAt) || (q.ClosedAt.Equal(after.ClosedAt) && q.ID < after.ID)) {
			continue
		}
		out = append(out, q)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeHistoryRepo) GetClosedQueue(_ context.Context, ownerID, queueID int64) (model.HistoryQueue, error) {
	if ownerID != f.owner {
		return model.HistoryQueue{}, apperror.ErrNotFound
	}
	for _, q := range f.queues {
		if q.ID == queueID {
			return q, nil
		}
	}
	return model.HistoryQueue{}, apperror.ErrNotFound
}

func (f *fakeHistoryRepo) ListQueueTickets(_ context.Context, _ int64, afterNumber, limit int) ([]model.HistoryTicket, error) {
	f.gotTicketCut, f.gotLimit = afterNumber, limit
	out := []model.HistoryTicket{}
	for _, t := range f.tickets {
		if t.TicketNumber > afterNumber {
			out = append(out, t)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// closedQueues membuat n antrian dengan closed_at menurun (id n paling lama).
func closedQueues(n int) []model.HistoryQueue {
	base := time.Date(2026, 11, 3, 12, 0, 0, 123456000, time.UTC)
	out := make([]model.HistoryQueue, n)
	for i := 0; i < n; i++ {
		out[i] = model.HistoryQueue{ID: int64(100 - i), QueueName: "Q", ClosedAt: base.Add(-time.Duration(i) * time.Hour)}
	}
	return out
}

func TestHistory_ListQueues_Pagination(t *testing.T) {
	repo := &fakeHistoryRepo{owner: 7, queues: closedQueues(5)}
	svc := service.NewHistoryService(repo)

	first, err := svc.ListQueues(context.Background(), 7, "", 2)
	require.NoError(t, err)
	require.Len(t, first.Queues, 2)
	assert.Equal(t, int64(100), first.Queues[0].ID)
	assert.NotEmpty(t, first.NextCursor, "masih ada halaman berikutnya")
	assert.Equal(t, 3, repo.gotLimit, "meminta satu baris lebih untuk mendeteksi halaman berikutnya")

	second, err := svc.ListQueues(context.Background(), 7, first.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, second.Queues, 2)
	assert.Equal(t, int64(98), second.Queues[0].ID, "lanjut tepat setelah baris terakhir halaman pertama")

	last, err := svc.ListQueues(context.Background(), 7, second.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, last.Queues, 1)
	assert.Empty(t, last.NextCursor, "halaman terakhir tanpa kursor")
}

func TestHistory_ListQueues_CursorKeepsMicrosecondPrecision(t *testing.T) {
	repo := &fakeHistoryRepo{owner: 7, queues: closedQueues(3)}
	svc := service.NewHistoryService(repo)

	page, err := svc.ListQueues(context.Background(), 7, "", 1)
	require.NoError(t, err)
	_, err = svc.ListQueues(context.Background(), 7, page.NextCursor, 1)
	require.NoError(t, err)

	require.NotNil(t, repo.gotAfter)
	assert.True(t, repo.gotAfter.ClosedAt.Equal(page.Queues[0].ClosedAt), "closed_at kursor sama persis, tanpa pembulatan")
	assert.Equal(t, page.Queues[0].ID, repo.gotAfter.ID)
}

func TestHistory_ListQueues_InvalidCursor(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7})
	for _, cursor := range []string{"abc", "1.2.3", "x.5", "5.x", "5.0", "5.-3"} {
		_, err := svc.ListQueues(context.Background(), 7, cursor, 10)
		assert.ErrorIs(t, err, apperror.ErrInvalidCursor, cursor)
	}
}

func TestHistory_ListQueues_LimitDefaultsAndClamp(t *testing.T) {
	repo := &fakeHistoryRepo{owner: 7}
	svc := service.NewHistoryService(repo)

	_, _ = svc.ListQueues(context.Background(), 7, "", 0)
	assert.Equal(t, service.DefaultHistoryQueuesLimit+1, repo.gotLimit)
	_, _ = svc.ListQueues(context.Background(), 7, "", 100000)
	assert.Equal(t, service.MaxHistoryQueuesLimit+1, repo.gotLimit)
}

func TestHistory_ListQueues_NonOwnerGetsEmptyList(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(3)})
	page, err := svc.ListQueues(context.Background(), 99, "", 10)
	require.NoError(t, err)
	assert.Empty(t, page.Queues)
	assert.Empty(t, page.NextCursor)
}

func TestHistory_GetQueue_NotFoundForNonOwnerOrMissing(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(2)})

	_, err := svc.GetQueue(context.Background(), 99, 100, "", 10)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "bukan owner")
	_, err = svc.GetQueue(context.Background(), 7, 555, "", 10)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "antrian tidak ada atau masih aktif")
}

func TestHistory_GetQueue_TicketPagination(t *testing.T) {
	tickets := []model.HistoryTicket{}
	for n := 1; n <= 5; n++ {
		tickets = append(tickets, model.HistoryTicket{TicketID: int64(n * 10), TicketNumber: n})
	}
	repo := &fakeHistoryRepo{owner: 7, queues: closedQueues(1), tickets: tickets}
	svc := service.NewHistoryService(repo)

	first, err := svc.GetQueue(context.Background(), 7, 100, "", 2)
	require.NoError(t, err)
	require.Len(t, first.Tickets, 2)
	assert.Equal(t, "2", first.NextCursor, "kursor nomor = nomor tiket terakhir di halaman")

	second, err := svc.GetQueue(context.Background(), 7, 100, first.NextCursor, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, second.Tickets[0].TicketNumber)
	assert.Equal(t, 2, repo.gotTicketCut)

	last, err := svc.GetQueue(context.Background(), 7, 100, second.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, last.Tickets, 1)
	assert.Empty(t, last.NextCursor)
}

func TestHistory_GetQueue_InvalidCursor(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1)})
	for _, cursor := range []string{"abc", "-1", "1.5"} {
		_, err := svc.GetQueue(context.Background(), 7, 100, cursor, 10)
		assert.ErrorIs(t, err, apperror.ErrInvalidCursor, cursor)
	}
}

// captureSink mencatat ukuran tiap halaman yang diterima.
type captureSink struct {
	batches []int
	numbers []int
	failAt  int
}

func (c *captureSink) WriteTickets(tickets []model.HistoryTicket) error {
	if c.failAt > 0 && len(c.batches)+1 == c.failAt {
		return assertErr
	}
	c.batches = append(c.batches, len(tickets))
	for _, t := range tickets {
		c.numbers = append(c.numbers, t.TicketNumber)
	}
	return nil
}

var assertErr = apperror.ErrRetryExhausted

func manyTickets(n int) []model.HistoryTicket {
	out := make([]model.HistoryTicket, n)
	for i := range out {
		out[i] = model.HistoryTicket{TicketID: int64(i + 1), TicketNumber: i + 1}
	}
	return out
}

func TestHistory_ExportQueue_OwnerAndClosedOnly(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1)})

	q, err := svc.ExportQueue(context.Background(), 7, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(100), q.ID)

	_, err = svc.ExportQueue(context.Background(), 99, 100)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "bukan owner")
	_, err = svc.ExportQueue(context.Background(), 7, 555)
	assert.ErrorIs(t, err, apperror.ErrNotFound, "tidak ada atau masih aktif")
}

func TestHistory_StreamTickets_PagesThroughEverythingInOrder(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1), tickets: manyTickets(1200)})
	sink := &captureSink{}

	require.NoError(t, svc.StreamTickets(context.Background(), 100, sink))
	assert.Equal(t, []int{500, 500, 200}, sink.batches, "dibaca per halaman 500, bukan sekaligus")
	require.Len(t, sink.numbers, 1200)
	for i, n := range sink.numbers {
		require.Equal(t, i+1, n, "urut nomor naik tanpa duplikat atau lompatan")
	}
}

func TestHistory_StreamTickets_ExactMultipleAndEmpty(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1), tickets: manyTickets(1000)})
	sink := &captureSink{}
	require.NoError(t, svc.StreamTickets(context.Background(), 100, sink))
	assert.Equal(t, []int{500, 500}, sink.batches, "kelipatan pas: tidak ada halaman kosong tambahan yang ditulis")

	empty := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1)})
	none := &captureSink{}
	require.NoError(t, empty.StreamTickets(context.Background(), 100, none))
	assert.Empty(t, none.batches)
}

func TestHistory_StreamTickets_SinkErrorStops(t *testing.T) {
	svc := service.NewHistoryService(&fakeHistoryRepo{owner: 7, queues: closedQueues(1), tickets: manyTickets(1200)})
	sink := &captureSink{failAt: 2}

	err := svc.StreamTickets(context.Background(), 100, sink)
	assert.Error(t, err)
	assert.Equal(t, []int{500}, sink.batches)
}
