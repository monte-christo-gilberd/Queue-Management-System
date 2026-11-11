package pusher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Sender: pengirim Web Push. Menyimpan kunci VAPID dan klien HTTP dengan batas waktu 8 detik.
type Sender struct {
	// publicKey: kunci publik VAPID.
	publicKey string
	// privateKey: kunci privat VAPID untuk menandatangani setiap push.
	privateKey string
	// subject: kontak pengirim (VAPID_SUBJECT).
	subject string
	// client: klien HTTP untuk memanggil push service. Batas waktu 8 detik per permintaan.
	client *http.Client
}

// New membuat Sender dengan kunci VAPID dan kontak pengirim. Klien HTTP punya batas waktu 8 detik.
func New(
	publicKey string,
	privateKey string,
	subject string,
) *Sender {
	// Susun Sender dengan kunci dan klien HTTP yang sudah diberi batas waktu.
	return &Sender{publicKey: publicKey, privateKey: privateKey, subject: subject, client: &http.Client{Timeout: 8 * time.Second}}
}

// Send mengirim satu push; kegagalan sementara dicoba ulang 3 kali (1s, 2s).
func (w *Sender) Send(
	ctx context.Context,
	sub model.Subscription,
	msg dto.Message,
) error {
	// Ubah pesan ke JSON sebagai isi push.
	body, err := json.Marshal(msg)
	// Gagal diubah: kembalikan galat, tidak ada yang bisa dikirim.
	if err != nil {
		return err
	}
	// Jalankan satu kali kirim, diulang hingga 3 kali bila galat sementara, dengan jeda awal 1 detik.
	return Retry(ctx, 3, time.Second, func() error { return w.once(ctx, body, sub) })
}

// once mengirim satu push tanpa retry, lalu menerjemahkan hasilnya ke ErrSubscriptionGone, ErrTemporaryPush, atau nil.
func (w *Sender) once(
	ctx context.Context,
	body []byte,
	sub model.Subscription,
) error {
	// Kirim notifikasi terenkripsi ke push service dengan header VAPID. TTL 60 detik karena pemberitahuan giliran
	// cepat basi bila browser offline. Urgency high agar segera diantar.
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
	// Permintaan gagal sebelum ada respons.
	if err != nil {
		// Konteks sudah batal: kembalikan galat konteks, jangan dicoba ulang.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Galat jaringan: tandai sebagai sementara agar dicoba ulang.
		return fmt.Errorf("%w: %v", apperror.ErrTemporaryPush, err)
	}
	// Tutup body respons setelah fungsi selesai.
	defer resp.Body.Close()
	// Pilih hasil berdasarkan status HTTP dari push service.
	switch {
	// 404 atau 410: browser sudah mencabut langganan. Kembalikan ErrSubscriptionGone supaya langganan dihapus.
	case resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound:
		return apperror.ErrSubscriptionGone
	// 429 atau 5xx: server sibuk atau rusak sementara, coba lagi nanti.
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: status %d", apperror.ErrTemporaryPush, resp.StatusCode)
	}
	// Status lain (sukses): tidak ada galat.
	return nil
}
