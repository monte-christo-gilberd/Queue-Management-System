// Package notify berisi logika inti: siapa yang diberi tahu untuk sebuah event.
package notify

import (
	"context"
	"fmt"

	"qms/notify-service/internal/event"
	"qms/notify-service/internal/message"
	"qms/notify-service/internal/push"
)

// Store: query baca plus hapus langganan yang kedaluwarsa.
type Store interface {
	Ticket(ctx context.Context, ticketID int64) (message.Ticket, error)
	// FirstWaiting: tiket waiting terdepan di antrian; nil bila tidak ada.
	FirstWaiting(ctx context.Context, queueID int64) (*message.Ticket, error)
	Subscriptions(ctx context.Context, userID int64) ([]message.Subscription, error)
	DeleteSubscription(ctx context.Context, endpoint string) error
}

type Sender interface {
	Send(ctx context.Context, sub message.Subscription, msg message.Message) error
}

type Service struct {
	store  Store
	sender Sender
}

func NewService(store Store, sender Sender) *Service {
	return &Service{store: store, sender: sender}
}

type Delivery struct {
	UserID int64
	Msg    message.Message
}

// Plan menghitung siapa yang diberi tahu. called: pemilik nomor ("Giliran Anda") dan tiket waiting terdepan
// ("Tinggal 1 nomor lagi"). recalled: hanya pemilik nomor.
func (s *Service) Plan(ctx context.Context, e event.Event) ([]Delivery, error) {
	t, err := s.store.Ticket(ctx, e.TicketID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("/ticket/%d", t.QueueID)
	out := []Delivery{{
		UserID: t.UserID,
		Msg:    message.Message{Title: t.QueueName, Body: fmt.Sprintf("Giliran Anda! Nomor %s dipanggil.", message.Label(t.Prefix, t.Number)), URL: url},
	}}
	if e.Kind == event.KindSkipped {
		body := fmt.Sprintf("Nomor %s dilewati.", message.Label(t.Prefix, t.Number))
		if t.Status == "waiting" {
			body = fmt.Sprintf("Nomor %s dilewati dan dikembalikan ke antrian.", message.Label(t.Prefix, t.Number))
		}
		out[0].Msg.Body = body
		return out, nil
	}
	if e.Kind != event.KindCalled {
		return out, nil
	}
	next, err := s.store.FirstWaiting(ctx, t.QueueID)
	if err != nil {
		return nil, err
	}
	if next != nil && next.UserID != t.UserID {
		out = append(out, Delivery{
			UserID: next.UserID,
			Msg:    message.Message{Title: t.QueueName, Body: fmt.Sprintf("Tinggal 1 nomor lagi. Nomor Anda %s berikutnya.", message.Label(next.Prefix, next.Number)), URL: url},
		})
	}
	return out, nil
}

// Handle mengirim semua push untuk satu event. Kegagalan satu perangkat tidak menghentikan yang lain.
func (s *Service) Handle(ctx context.Context, e event.Event) error {
	plan, err := s.Plan(ctx, e)
	if err != nil {
		return err
	}
	var firstErr error
	for _, d := range plan {
		subs, err := s.store.Subscriptions(ctx, d.UserID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, sub := range subs {
			err := s.sender.Send(ctx, sub, d.Msg)
			switch {
			case err == nil:
			case err == push.ErrGone:
				_ = s.store.DeleteSubscription(ctx, sub.Endpoint)
			default:
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}
