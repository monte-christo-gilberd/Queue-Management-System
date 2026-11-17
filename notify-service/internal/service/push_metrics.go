package service

import (
	"context"
	"errors"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"
)

// PushCounters menerima hasil kirim push untuk metrik. Dipenuhi metrics.Notify.
type PushCounters interface {
	// IncSent dipanggil untuk push yang berhasil dikirim; channel adalah "webpush" atau "fcm".
	IncSent(channel string)
	// IncFailed dipanggil untuk push yang gagal; reason adalah "gone" atau "error".
	IncFailed(channel, reason string)
}

// meteredSender membungkus PushSender dan menghitung hasilnya, tanpa mengubah perilaku pengiriman.
type meteredSender struct {
	PushSender
	counters PushCounters
}

// NewMeteredPushSender membungkus pengirim push dengan penghitung metrik. Hasil dihitung per perangkat, setelah
// retry di dalam pengirim selesai.
func NewMeteredPushSender(
	sender PushSender,
	counters PushCounters,
) PushSender {
	// tanpa penghitung, kembalikan pengirim apa adanya (tidak ada lapisan tambahan)
	if counters == nil {
		return sender
	}
	return &meteredSender{PushSender: sender, counters: counters}
}

// Send mengirim satu push lalu menghitung hasilnya.
func (m *meteredSender) Send(
	ctx context.Context,
	sub model.Subscription,
	msg dto.Message,
) error {
	// teruskan ke pengirim asli; hasil dan galatnya dikembalikan apa adanya di akhir
	err := m.PushSender.Send(ctx, sub, msg)
	// kanal pengiriman (webpush atau fcm) menjadi label metrik
	channel := sub.Channel()
	// klasifikasikan hasil untuk metrik
	switch {
	// berhasil
	case err == nil:
		m.counters.IncSent(channel)
	// langganan sudah tidak berlaku (404/410, UNREGISTERED): dibedakan dari galat lain karena wajar terjadi
	case errors.Is(err, apperror.ErrSubscriptionGone):
		m.counters.IncFailed(channel, "gone")
	// galat lain
	default:
		m.counters.IncFailed(channel, "error")
	}
	return err
}
