package pusher

import (
	"context"
	"errors"

	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"
)

// ErrChannelDisabled: langganan berjenis FCM tetapi pengirim FCM tidak dipasang (kredensial kosong). Dalam praktik
// jarang terjadi, karena repository tidak membaca push_devices saat FCM mati (NotifyRepository.EnableDevices).
var ErrChannelDisabled = errors.New("push channel disabled")

// ChannelSender: pengirim untuk satu jenis langganan. Dipenuhi pusher.Sender (Web Push) dan fcm.Sender (FCM).
// Dideklarasikan di sini, bukan mengimpor package fcm, supaya tidak ada impor melingkar (fcm memakai pusher.Retry).
type ChannelSender interface {
	// Send mengirim satu notifikasi ke satu langganan.
	Send(ctx context.Context, sub model.Subscription, msg dto.Message) error
}

// Dispatcher meneruskan setiap langganan ke pengirim sesuai jenisnya (model.Subscription.Channel). Bentuknya sama
// dengan service.PushSender, sehingga NotifyService dan pembungkus metrik tidak berubah.
type Dispatcher struct {
	// webPush: pengirim Web Push ke browser. Selalu ada.
	webPush ChannelSender
	// fcm: pengirim FCM ke aplikasi Android. nil bila kredensial FCM tidak dipasang.
	fcm ChannelSender
}

// NewDispatcher membuat Dispatcher. fcm nil berarti langganan FCM ditolak dengan ErrChannelDisabled, sedangkan
// Web Push tetap berjalan.
func NewDispatcher(webPush, fcm ChannelSender) *Dispatcher {
	return &Dispatcher{webPush: webPush, fcm: fcm}
}

// Send mengirim ke pengirim yang cocok dengan jenis langganan.
func (d *Dispatcher) Send(ctx context.Context, sub model.Subscription, msg dto.Message) error {
	// Token FCM perangkat Android.
	if sub.Channel() == model.KindFCM {
		// FCM mati: jangan sampai panik karena pengirim nil.
		if d.fcm == nil {
			return ErrChannelDisabled
		}
		return d.fcm.Send(ctx, sub, msg)
	}
	// Selain FCM (termasuk Kind kosong dari data lama): Web Push.
	return d.webPush.Send(ctx, sub, msg)
}
