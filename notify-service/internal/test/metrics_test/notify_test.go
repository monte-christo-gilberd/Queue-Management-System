package metrics_test

import (
	"context"
	"errors"
	"testing"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/metrics"
	"qms/notify-service/internal/model"
	"qms/notify-service/internal/service"
)

// fakeSender memenuhi service.PushSender dengan hasil tetap, tanpa menghubungi push service sungguhan.
type fakeSender struct{ err error }

// Send mengembalikan galat yang diatur (nil = berhasil).
func (f fakeSender) Send(context.Context, model.Subscription, dto.Message) error { return f.err }

// Hasil kirim push dihitung: berhasil, langganan hilang (gone), dan galat lain; galat diteruskan apa adanya
// (pembungkus metrik tidak boleh mengubah perilaku pengiriman).
func TestMeteredPushSender(t *testing.T) {
	m := metrics.NewNotify()
	ctx := context.Background()
	other := errors.New("boom")

	// empat kiriman: sukses, langganan hilang, dan dua galat lain
	for _, tc := range []struct {
		err  error
		want error
	}{{nil, nil}, {apperror.ErrSubscriptionGone, apperror.ErrSubscriptionGone}, {other, other}, {other, other}} {
		got := service.NewMeteredPushSender(fakeSender{err: tc.err}, m).Send(ctx, model.Subscription{}, dto.Message{})
		// galat yang dikembalikan harus sama dengan galat pengirim asli
		if !errors.Is(got, tc.want) && got != tc.want {
			t.Fatalf("galat diubah: dapat %v, diharapkan %v", got, tc.want)
		}
	}

	// satu terkirim
	if got := counter(t, m.Registry, "push_sent_total", map[string]string{"channel": "webpush"}); got != 1 {
		t.Errorf("push_sent_total = %v, diharapkan 1", got)
	}
	// satu gone (wajar: browser menghapus langganan)
	if got := counter(t, m.Registry, "push_failed_total", map[string]string{"channel": "webpush", "reason": "gone"}); got != 1 {
		t.Errorf("failed{gone} = %v, diharapkan 1", got)
	}
	// dua galat lain
	if got := counter(t, m.Registry, "push_failed_total", map[string]string{"channel": "webpush", "reason": "error"}); got != 2 {
		t.Errorf("failed{error} = %v, diharapkan 2", got)
	}
}

// Push FCM dihitung di label channel="fcm", terpisah dari Web Push.
func TestMeteredPushSenderSplitsChannel(t *testing.T) {
	m := metrics.NewNotify()
	ctx := context.Background()
	// satu kiriman FCM berhasil dan satu token FCM mati
	fcmSub := model.Subscription{Kind: model.KindFCM, Token: "tok"}
	_ = service.NewMeteredPushSender(fakeSender{}, m).Send(ctx, fcmSub, dto.Message{})
	_ = service.NewMeteredPushSender(fakeSender{err: apperror.ErrSubscriptionGone}, m).Send(ctx, fcmSub, dto.Message{})

	if got := counter(t, m.Registry, "push_sent_total", map[string]string{"channel": "fcm"}); got != 1 {
		t.Errorf("sent{fcm} = %v, diharapkan 1", got)
	}
	// tidak ada yang tercatat di kanal Web Push
	if got := counter(t, m.Registry, "push_sent_total", map[string]string{"channel": "webpush"}); got != 0 {
		t.Errorf("sent{webpush} = %v, diharapkan 0", got)
	}
	if got := counter(t, m.Registry, "push_failed_total", map[string]string{"channel": "fcm", "reason": "gone"}); got != 1 {
		t.Errorf("failed{fcm,gone} = %v, diharapkan 1", got)
	}
}
