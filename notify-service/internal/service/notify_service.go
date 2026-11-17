package service

import (
	"context"
	"errors"
	"fmt"

	"qms/notify-service/internal/apperror"
	"qms/notify-service/internal/dto"
	"qms/notify-service/internal/model"
)

// NotifyRepository: query baca plus hapus langganan yang kedaluwarsa. Dipenuhi repository.NotifyRepository.
type NotifyRepository interface {
	// Ticket: data tiket beserta antrian dan nama counter. Galat bila tiket tidak ada.
	Ticket(
		ctx context.Context,
		ticketID int64,
	) (model.Ticket, error)
	// FirstWaiting: tiket waiting terdepan di antrian; nil bila tidak ada.
	FirstWaiting(
		ctx context.Context,
		queueID int64,
	) (*model.Ticket, error)
	// PositionOf: posisi tiket di urutan tunggu (1 = terdepan).
	PositionOf(
		ctx context.Context,
		ticketID int64,
	) (int, error)
	// Subscriptions: semua langganan push milik user.
	Subscriptions(
		ctx context.Context,
		userID int64,
	) ([]model.Subscription, error)
	// DeleteSubscription: menghapus langganan yang sudah tidak berlaku, dari tabel sesuai jenisnya.
	DeleteSubscription(
		ctx context.Context,
		sub model.Subscription,
	) error
}

// PushSender: pengirim push ke satu langganan. Dipenuhi pusher.Dispatcher (Web Push dan FCM) atau pusher.Sender.
type PushSender interface {
	// Send mengirim satu notifikasi ke satu langganan.
	Send(
		ctx context.Context,
		sub model.Subscription,
		msg dto.Message,
	) error
}

// NotifyService: logika notifikasi. Menentukan penerima dan mengirim push.
type NotifyService struct {
	// repo: sumber data tiket dan langganan.
	repo NotifyRepository
	// sender: pengirim push ke browser.
	sender PushSender
}

// NewNotifyService membuat NotifyService dengan repo dan sender yang sudah siap.
func NewNotifyService(
	repo NotifyRepository,
	sender PushSender,
) *NotifyService {
	// Simpan kedua dependensi ke Service.
	return &NotifyService{repo: repo, sender: sender}
}

// Delivery: satu tujuan pengiriman: penerima dan isi pesannya.
type Delivery struct {
	// UserID: id user penerima.
	UserID int64
	// Msg: isi pesan yang dikirim ke penerima ini.
	Msg dto.Message
}

// Plan menghitung siapa yang diberi tahu. called: pemilik nomor ("Giliran Anda") dan tiket waiting terdepan
// ("Tinggal 1 nomor lagi"). recalled: hanya pemilik nomor.
// requeued: hanya pemilik nomor, dengan posisi baru; skipped: hanya skip permanen.
func (s *NotifyService) Plan(
	ctx context.Context,
	e model.Event,
) ([]Delivery, error) {
	// Baca data tiket yang terkait event.
	t, err := s.repo.Ticket(ctx, e.TicketID)
	// Galat (mis. tiket tidak ada) membatalkan perencanaan.
	if err != nil {
		return nil, err
	}
	// Tautan notifikasi mengarah ke halaman antrian milik tiket ini.
	url := fmt.Sprintf("/ticket/%d", t.QueueID)
	// Teks default untuk panggilan: giliran pemilik nomor, tanpa nama counter.
	called := fmt.Sprintf("Giliran Anda! Nomor %s dipanggil.", Label(t.Prefix, t.Number))
	// Counter pemanggil diketahui: sebut nama counter di teks.
	if t.CounterName != "" {
		called = fmt.Sprintf("Giliran Anda! Nomor %s dipanggil di %s.", Label(t.Prefix, t.Number), t.CounterName)
	}
	// Penerima pertama selalu pemilik nomor, dengan teks panggilan.
	out := []Delivery{{
		UserID: t.UserID,
		Msg:    dto.Message{Title: t.QueueName, Body: called, URL: url},
	}}
	// Nomor dilewati permanen: ganti isi pesan dengan teks skip, lalu selesai (tidak ada penerima lain).
	if e.Kind == model.KindSkipped {
		out[0].Msg.Body = fmt.Sprintf("Nomor %s dilewati.", Label(t.Prefix, t.Number))
		return out, nil
	}
	// Nomor kembali menunggu: hitung posisi baru pemilik nomor, lalu pilih teks sesuai alasannya.
	if e.Kind == model.KindRequeued {
		// Label nomor tiket untuk ditampilkan di teks.
		label := Label(t.Prefix, t.Number)
		// Ambil posisi baru tiket di urutan tunggu.
		pos, err := s.repo.PositionOf(ctx, e.TicketID)
		// Galat membatalkan perencanaan.
		if err != nil {
			return nil, err
		}
		// Pilih teks berdasarkan alasan dan nama counter.
		switch {
		// Counter dihapus dan namanya ada di payload: sebut nama counter itu.
		case e.Reason == model.ReasonCounterClosed && e.Counter != "":
			out[0].Msg.Body = fmt.Sprintf("%s ditutup. Nomor %s kembali ke antrian, posisi Anda sekarang ke-%d.", e.Counter, label, pos)
		// Counter dihapus tetapi namanya tidak ada: pakai teks umum.
		case e.Reason == model.ReasonCounterClosed:
			out[0].Msg.Body = fmt.Sprintf("Counter ditutup. Nomor %s kembali ke antrian, posisi Anda sekarang ke-%d.", label, pos)
		// Alasan lain (skip dengan fallback): nomor dilewati lalu dikembalikan.
		default:
			out[0].Msg.Body = fmt.Sprintf("Nomor %s dilewati dan dikembalikan ke antrian, posisi Anda sekarang ke-%d.", label, pos)
		}
		return out, nil
	}
	// Jenis lain selain called (mis. recalled): cukup kirim ke pemilik nomor.
	if e.Kind != model.KindCalled {
		return out, nil
	}
	// Nomor dipanggil: cari tiket waiting terdepan, untuk memberi tahu bahwa giliran dia sudah dekat.
	next, err := s.repo.FirstWaiting(ctx, t.QueueID)
	// Galat membaca antrian membatalkan perencanaan.
	if err != nil {
		return nil, err
	}
	// Ada tiket waiting terdepan dan itu milik orang lain: beri tahu dia bahwa tinggal satu nomor lagi.
	if next != nil && next.UserID != t.UserID {
		out = append(out, Delivery{
			UserID: next.UserID,
			Msg:    dto.Message{Title: t.QueueName, Body: fmt.Sprintf("Tinggal 1 nomor lagi. Nomor Anda %s berikutnya.", Label(next.Prefix, next.Number)), URL: url},
		})
	}
	return out, nil
}

// Handle mengirim semua push untuk satu event. Kegagalan satu perangkat tidak menghentikan yang lain.
func (s *NotifyService) Handle(
	ctx context.Context,
	e model.Event,
) error {
	// Tentukan penerima dan isi pesan.
	plan, err := s.Plan(ctx, e)
	// Galat di tahap ini membatalkan seluruh event, karena belum ada yang terkirim.
	if err != nil {
		return err
	}
	// Simpan galat pertama agar bisa dikembalikan, tanpa menghentikan pengiriman lain.
	var firstErr error
	// Kirim ke setiap penerima.
	for _, d := range plan {
		// Ambil semua langganan push penerima ini.
		subs, err := s.repo.Subscriptions(ctx, d.UserID)
		// Gagal membaca langganan: catat galat pertama, lalu lanjut ke penerima berikutnya.
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// Kirim ke setiap perangkat (langganan) milik penerima.
		for _, sub := range subs {
			// Kirim satu push ke satu perangkat.
			err := s.sender.Send(ctx, sub, d.Msg)
			// Pilih tindakan sesuai hasil kirim.
			switch {
			// Berhasil terkirim: tidak perlu tindakan.
			case err == nil:
			// Langganan sudah tidak berlaku (Web Push 404/410, FCM UNREGISTERED): hapus dari database supaya tidak
			// dikirimi lagi. Galat hapus diabaikan karena akan dicoba lagi pada pengiriman berikutnya.
			case errors.Is(err, apperror.ErrSubscriptionGone):
				_ = s.repo.DeleteSubscription(ctx, sub)
			// Galat lain: simpan galat pertama, lalu lanjut ke perangkat berikutnya.
			default:
				// Hanya galat pertama yang disimpan.
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	// Kembalikan galat pertama (bila ada) setelah semua penerima diproses.
	return firstErr
}
