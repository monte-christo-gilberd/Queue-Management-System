// Package push mengirim Web Push lewat VAPID.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"qms/notify-service/internal/message"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// ErrGone: langganan sudah tidak berlaku (HTTP 404/410 dari push service).
var ErrGone = errors.New("subscription gone")

type Sender struct {
	publicKey  string
	privateKey string
	subject    string
}

func New(publicKey, privateKey, subject string) *Sender {
	return &Sender{publicKey: publicKey, privateKey: privateKey, subject: subject}
}

func (w *Sender) Send(ctx context.Context, sub message.Subscription, msg message.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber:      w.subject,
		VAPIDPublicKey:  w.publicKey,
		VAPIDPrivateKey: w.privateKey,
		TTL:             60,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		return ErrGone
	}
	return nil
}
