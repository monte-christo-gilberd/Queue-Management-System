// Package push mengirim Web Push lewat VAPID.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"qms/notify-service/internal/message"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// ErrGone: langganan sudah tidak berlaku (HTTP 404/410 dari push service).
var ErrGone = errors.New("subscription gone")

type Sender struct {
	publicKey  string
	privateKey string
	subject    string
	client     *http.Client
}

func New(publicKey, privateKey, subject string) *Sender {
	return &Sender{publicKey: publicKey, privateKey: privateKey, subject: subject, client: &http.Client{Timeout: 8 * time.Second}}
}

// Send mengirim satu push; kegagalan sementara dicoba ulang 3 kali (1s, 2s).
func (w *Sender) Send(ctx context.Context, sub message.Subscription, msg message.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return Retry(ctx, 3, time.Second, func() error { return w.once(ctx, body, sub) })
}

func (w *Sender) once(ctx context.Context, body []byte, sub message.Subscription) error {
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      w.client,
		Subscriber:      w.subject,
		VAPIDPublicKey:  w.publicKey,
		VAPIDPrivateKey: w.privateKey,
		TTL:             60,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrTemporary, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound:
		return ErrGone
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: status %d", ErrTemporary, resp.StatusCode)
	}
	return nil
}
