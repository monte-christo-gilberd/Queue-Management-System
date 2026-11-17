package service_test

import (
	"context"
	"errors"
	"testing"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"
	"qms/notify-service/internal/service"
)

type fakeStore struct {
	ticket  model.Ticket
	first   *model.Ticket
	pos     int
	subs    map[int64][]model.Subscription
	deleted []string
}

func (f *fakeStore) Ticket(context.Context, int64) (model.Ticket, error) { return f.ticket, nil }
func (f *fakeStore) FirstWaiting(context.Context, int64) (*model.Ticket, error) {
	return f.first, nil
}
func (f *fakeStore) PositionOf(context.Context, int64) (int, error) { return f.pos, nil }
func (f *fakeStore) Subscriptions(_ context.Context, u int64) ([]model.Subscription, error) {
	return f.subs[u], nil
}
func (f *fakeStore) DeleteSubscription(_ context.Context, s model.Subscription) error {
	f.deleted = append(f.deleted, s.Endpoint)
	return nil
}

type fakeSender struct {
	sent []string
	errs map[string]error
}

func (f *fakeSender) Send(_ context.Context, s model.Subscription, m dto.Message) error {
	if err := f.errs[s.Endpoint]; err != nil {
		return err
	}
	f.sent = append(f.sent, s.Endpoint+"|"+m.Body)
	return nil
}

func sub(e string) model.Subscription { return model.Subscription{Endpoint: e} }

func TestCalledNotifiesOwnerAndNextWaiting(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q"},
		first:  &model.Ticket{ID: 2, Number: 2, UserID: 20},
		subs:   map[int64][]model.Subscription{10: {sub("a")}, 20: {sub("b")}},
	}
	sd := &fakeSender{}
	if err := service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(sd.sent) != 2 {
		t.Fatalf("want 2 pushes, got %v", sd.sent)
	}
}

func TestCalledSameUserNotDuplicated(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		first:  &model.Ticket{ID: 2, Number: 2, UserID: 10},
		subs:   map[int64][]model.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	_ = service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "called", QueueID: 9, TicketID: 1})
	if len(sd.sent) != 1 {
		t.Fatalf("want 1 push, got %v", sd.sent)
	}
}

func TestRecalledOnlyOwner(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		first:  &model.Ticket{ID: 2, Number: 2, UserID: 20},
		subs:   map[int64][]model.Subscription{10: {sub("a")}, 20: {sub("b")}},
	}
	sd := &fakeSender{}
	_ = service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "recalled", QueueID: 9, TicketID: 1})
	if len(sd.sent) != 1 {
		t.Fatalf("want 1 push, got %v", sd.sent)
	}
}

func TestGoneDeletedAndOthersStillSent(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		subs:   map[int64][]model.Subscription{10: {sub("dead"), sub("bad"), sub("ok")}},
	}
	sd := &fakeSender{errs: map[string]error{"dead": apperror.ErrSubscriptionGone, "bad": errors.New("boom")}}
	err := service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "called", QueueID: 9, TicketID: 1})
	if err == nil {
		t.Fatal("want error from failed device")
	}
	if len(st.deleted) != 1 || st.deleted[0] != "dead" {
		t.Fatalf("deleted=%v", st.deleted)
	}
	if len(sd.sent) != 1 {
		t.Fatalf("sent=%v", sd.sent)
	}
}

func TestSkippedPermanentNotifiesOnlyOwner(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 3, UserID: 10, Status: "skipped"},
		first:  &model.Ticket{ID: 2, Number: 4, UserID: 20},
		subs:   map[int64][]model.Subscription{10: {sub("a")}, 20: {sub("b")}},
	}
	sd := &fakeSender{}
	_ = service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "skipped", QueueID: 9, TicketID: 1})
	if len(sd.sent) != 1 || sd.sent[0] != "a|Nomor A-003 dilewati." {
		t.Fatalf("got %v", sd.sent)
	}
}

func TestRequeuedTextsAndOnlyOwner(t *testing.T) {
	cases := map[string]struct {
		ev   model.Event
		want string
	}{
		"skip": {model.Event{Kind: "requeued", QueueID: 9, TicketID: 1, Reason: "skip"},
			"a|Nomor A-003 dilewati dan dikembalikan ke antrian, posisi Anda sekarang ke-3."},
		"counter named": {model.Event{Kind: "requeued", QueueID: 9, TicketID: 1, Reason: "counter_closed", Counter: "Counter 2"},
			"a|Counter 2 ditutup. Nomor A-003 kembali ke antrian, posisi Anda sekarang ke-3."},
		"counter unnamed": {model.Event{Kind: "requeued", QueueID: 9, TicketID: 1, Reason: "counter_closed"},
			"a|Counter ditutup. Nomor A-003 kembali ke antrian, posisi Anda sekarang ke-3."},
	}
	for name, c := range cases {
		st := &fakeStore{
			ticket: model.Ticket{ID: 1, QueueID: 9, Number: 3, UserID: 10, Status: "waiting"},
			first:  &model.Ticket{ID: 2, Number: 4, UserID: 20},
			pos:    3,
			subs:   map[int64][]model.Subscription{10: {sub("a")}, 20: {sub("b")}},
		}
		sd := &fakeSender{}
		if err := service.NewNotifyService(st, sd).Handle(context.Background(), c.ev); err != nil {
			t.Fatal(err)
		}
		if len(sd.sent) != 1 || sd.sent[0] != c.want {
			t.Fatalf("%s: got %v", name, sd.sent)
		}
	}
}

func TestCalledNamesCounterWhenQueueHasSeveral(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q", CounterName: "Counter 2"},
		subs:   map[int64][]model.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	if err := service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	want := "a|Giliran Anda! Nomor A-001 dipanggil di Counter 2."
	if len(sd.sent) != 1 || sd.sent[0] != want {
		t.Fatalf("want [%q], got %v", want, sd.sent)
	}
}

func TestCalledWithoutCounterNameKeepsOriginalText(t *testing.T) {
	st := &fakeStore{
		ticket: model.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q"},
		subs:   map[int64][]model.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	if err := service.NewNotifyService(st, sd).Handle(context.Background(), model.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	want := "a|Giliran Anda! Nomor A-001 dipanggil."
	if len(sd.sent) != 1 || sd.sent[0] != want {
		t.Fatalf("want [%q], got %v", want, sd.sent)
	}
}
