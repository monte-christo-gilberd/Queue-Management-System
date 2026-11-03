package notify_test

import (
	"context"
	"errors"
	"testing"

	"qms/notify-service/internal/event"
	"qms/notify-service/internal/message"
	"qms/notify-service/internal/notify"
	"qms/notify-service/internal/push"
)

type fakeStore struct {
	ticket  message.Ticket
	first   *message.Ticket
	subs    map[int64][]message.Subscription
	deleted []string
}

func (f *fakeStore) Ticket(context.Context, int64) (message.Ticket, error) { return f.ticket, nil }
func (f *fakeStore) FirstWaiting(context.Context, int64) (*message.Ticket, error) {
	return f.first, nil
}
func (f *fakeStore) Subscriptions(_ context.Context, u int64) ([]message.Subscription, error) {
	return f.subs[u], nil
}
func (f *fakeStore) DeleteSubscription(_ context.Context, e string) error {
	f.deleted = append(f.deleted, e)
	return nil
}

type fakeSender struct {
	sent []string
	errs map[string]error
}

func (f *fakeSender) Send(_ context.Context, s message.Subscription, m message.Message) error {
	if err := f.errs[s.Endpoint]; err != nil {
		return err
	}
	f.sent = append(f.sent, s.Endpoint+"|"+m.Body)
	return nil
}

func sub(e string) message.Subscription { return message.Subscription{Endpoint: e} }

func TestCalledNotifiesOwnerAndNextWaiting(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q"},
		first:  &message.Ticket{ID: 2, Number: 2, UserID: 20},
		subs:   map[int64][]message.Subscription{10: {sub("a")}, 20: {sub("b")}},
	}
	sd := &fakeSender{}
	if err := notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(sd.sent) != 2 {
		t.Fatalf("want 2 pushes, got %v", sd.sent)
	}
}

func TestCalledSameUserNotDuplicated(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		first:  &message.Ticket{ID: 2, Number: 2, UserID: 10},
		subs:   map[int64][]message.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	_ = notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "called", QueueID: 9, TicketID: 1})
	if len(sd.sent) != 1 {
		t.Fatalf("want 1 push, got %v", sd.sent)
	}
}

func TestRecalledOnlyOwner(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		first:  &message.Ticket{ID: 2, Number: 2, UserID: 20},
		subs:   map[int64][]message.Subscription{10: {sub("a")}, 20: {sub("b")}},
	}
	sd := &fakeSender{}
	_ = notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "recalled", QueueID: 9, TicketID: 1})
	if len(sd.sent) != 1 {
		t.Fatalf("want 1 push, got %v", sd.sent)
	}
}

func TestGoneDeletedAndOthersStillSent(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10},
		subs:   map[int64][]message.Subscription{10: {sub("dead"), sub("bad"), sub("ok")}},
	}
	sd := &fakeSender{errs: map[string]error{"dead": push.ErrGone, "bad": errors.New("boom")}}
	err := notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "called", QueueID: 9, TicketID: 1})
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

func TestSkippedNotifiesOnlyOwnerWithStatusText(t *testing.T) {
	for status, want := range map[string]string{
		"skipped": "Nomor A-003 dilewati.",
		"waiting": "Nomor A-003 dilewati dan dikembalikan ke antrian.",
	} {
		st := &fakeStore{
			ticket: message.Ticket{ID: 1, QueueID: 9, Number: 3, UserID: 10, Status: status},
			first:  &message.Ticket{ID: 2, Number: 4, UserID: 20},
			subs:   map[int64][]message.Subscription{10: {sub("a")}, 20: {sub("b")}},
		}
		sd := &fakeSender{}
		_ = notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "skipped", QueueID: 9, TicketID: 1})
		if len(sd.sent) != 1 || sd.sent[0] != "a|"+want {
			t.Fatalf("status %s: got %v", status, sd.sent)
		}
	}
}

func TestCalledNamesCounterWhenQueueHasSeveral(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q", CounterName: "Counter 2"},
		subs:   map[int64][]message.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	if err := notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	want := "a|Giliran Anda! Nomor A-001 dipanggil di Counter 2."
	if len(sd.sent) != 1 || sd.sent[0] != want {
		t.Fatalf("want [%q], got %v", want, sd.sent)
	}
}

func TestCalledWithoutCounterNameKeepsOriginalText(t *testing.T) {
	st := &fakeStore{
		ticket: message.Ticket{ID: 1, QueueID: 9, Number: 1, UserID: 10, QueueName: "Q"},
		subs:   map[int64][]message.Subscription{10: {sub("a")}},
	}
	sd := &fakeSender{}
	if err := notify.NewService(st, sd).Handle(context.Background(), event.Event{Kind: "called", QueueID: 9, TicketID: 1}); err != nil {
		t.Fatal(err)
	}
	want := "a|Giliran Anda! Nomor A-001 dipanggil."
	if len(sd.sent) != 1 || sd.sent[0] != want {
		t.Fatalf("want [%q], got %v", want, sd.sent)
	}
}
