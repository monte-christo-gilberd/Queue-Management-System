package notify

import (
	"context"
	"fmt"

	"qms/notify-service/internal/event"
	"qms/notify-service/internal/message"
	"qms/notify-service/internal/push"
)

// Store: query baca plus hapus langganan yang kedaluwarsa. Dipenuhi package store.
type Store interface {
	// Ticket: data tiket beserta antrian dan nama counter. Galat bila tiket tidak ada.
	Ticket(ctx context.Context, ticketID int64) (message.Ticket, error)
	// FirstWaiting: tiket waiting terdepan di antrian; nil bila tidak ada.
	FirstWaiting(ctx context.Context, queueID int64) (*message.Ticket, error)
	// PositionOf: posisi tiket di urutan tunggu (1 = terdepan).
	PositionOf(ctx context.Context, ticketID int64) (int, error)
	// Subscriptions: semua langganan push milik user.
	Subscriptions(ctx context.Context, userID int64) ([]message.Subscription, error)
	// DeleteSubscription: menghapus langganan yang sudah tidak berlaku.
	DeleteSubscription(ctx context.Context, endpoint string) error
}

// Sender: pengirim push ke satu langganan. Dipenuhi push.Sender.
type Sender interface {
	// Send mengirim satu notifikasi ke satu langganan.
	Send(ctx context.Context, sub message.Subscription, msg message.Message) error
}

// Service: logika notifikasi. Menentukan penerima dan mengirim push.
type Service struct {
	// store: sumber data tiket dan langganan.
	store Store
	// sender: pengirim push ke browser.
	sender Sender
}

// NewService membuat Service dengan store dan sender yang sudah siap.
func NewService(store Store, sender Sender) *Service {
	// Simpan kedua dependensi ke Service.
	return &Service{store: store, sender: sender}
}

// Delivery: satu tujuan pengiriman: penerima dan isi pesannya.
type Delivery struct {
	// UserID: id user penerima.
	UserID int64
	// Msg: isi pesan yang dikirim ke penerima ini.
	Msg message.Message
}

// Plan menghitung siapa yang diberi tahu. called: pemilik nomor ("Giliran Anda") dan tiket waiting terdepan
// ("Tinggal 1 nomor lagi"). recalled: hanya pemilik nomor.
// requeued: hanya pemilik nomor, dengan posisi baru; skipped: hanya skip permanen.
func (s *Service) Plan(ctx context.Context, e event.Event) ([]Delivery, error) {
	// Baca data tiket yang terkait event. Galat (mis. tiket tidak ada) membatalkan perencanaan.
	t, err := s.store.Ticket(ctx, e.TicketID)
	if err != nil {
		return nil, err
	}
	// Tautan notifikasi mengarah ke halaman antrian milik tiket ini.
	url := fmt.Sprintf("/ticket/%d", t.QueueID)
	// Teks default untuk panggilan: giliran pemilik nomor, tanpa nama counter.
	called := fmt.Sprintf("Giliran Anda! Nomor %s dipanggil.", message.Label(t.Prefix, t.Number))
	// Counter pemanggil diketahui: sebut nama counter di teks.
	if t.CounterName != "" {
		called = fmt.Sprintf("Giliran Anda! Nomor %s dipanggil di %s.", message.Label(t.Prefix, t.Number), t.CounterName)
	}
	// Penerima pertama selalu pemilik nomor, dengan teks panggilan.
	out := []Delivery{{
		UserID: t.UserID,
		Msg:    message.Message{Title: t.QueueName, Body: called, URL: url},
	}}
	// Nomor dilewati permanen: ganti isi pesan dengan teks skip, lalu selesai (tidak ada penerima lain).
	if e.Kind == event.KindSkipped {
		out[0].Msg.Body = fmt.Sprintf("Nomor %s dilewati.", message.Label(t.Prefix, t.Number))
		return out, nil
	}
	// Nomor kembali menunggu: hitung posisi baru pemilik nomor, lalu pilih teks sesuai alasannya.
	if e.Kind == event.KindRequeued {
		// Label nomor tiket untuk ditampilkan di teks.
		label := message.Label(t.Prefix, t.Number)
		// Ambil posisi baru tiket di urutan tunggu. Galat membatalkan perencanaan.
		pos, err := s.store.PositionOf(ctx, e.TicketID)
		if err != nil {
			return nil, err
		}
		// Pilih teks berdasarkan alasan dan nama counter.
		switch {
		// Counter dihapus dan namanya ada di payload: sebut nama counter itu.
		case e.Reason == event.ReasonCounterClosed && e.Counter != "":
			out[0].Msg.Body = fmt.Sprintf("%s ditutup. Nomor %s kembali ke antrian, posisi Anda sekarang ke-%d.", e.Counter, label, pos)
		// Counter dihapus tetapi namanya tidak ada: pakai teks umum.
		case e.Reason == event.ReasonCounterClosed:
			out[0].Msg.Body = fmt.Sprintf("Counter ditutup. Nomor %s kembali ke antrian, posisi Anda sekarang ke-%d.", label, pos)
		// Alasan lain (skip dengan fallback): nomor dilewati lalu dikembalikan.
		default:
			out[0].Msg.Body = fmt.Sprintf("Nomor %s dilewati dan dikembalikan ke antrian, posisi Anda sekarang ke-%d.", label, pos)
		}
		return out, nil
	}
	// Jenis lain selain called (mis. recalled): cukup kirim ke pemilik nomor.
	if e.Kind != event.KindCalled {
		return out, nil
	}
	// Nomor dipanggil: cari tiket waiting terdepan, untuk memberi tahu bahwa giliran dia sudah dekat.
	next, err := s.store.FirstWaiting(ctx, t.QueueID)
	if err != nil {
		return nil, err
	}
	// Ada tiket waiting terdepan dan itu milik orang lain: beri tahu dia bahwa tinggal satu nomor lagi.
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
	// Tentukan penerima dan isi pesan. Galat di tahap ini membatalkan seluruh event.
	plan, err := s.Plan(ctx, e)
	if err != nil {
		return err
	}
	// Simpan galat pertama agar bisa dikembalikan, tanpa menghentikan pengiriman lain.
	var firstErr error
	// Kirim ke setiap penerima.
	for _, d := range plan {
		// Ambil semua langganan push penerima ini.
		subs, err := s.store.Subscriptions(ctx, d.UserID)
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
			// Langganan sudah tidak berlaku (404/410): hapus dari database. Galat hapus diabaikan.
			case err == push.ErrGone:
				_ = s.store.DeleteSubscription(ctx, sub.Endpoint)
			// Galat lain: simpan galat pertama, lalu lanjut ke perangkat berikutnya.
			default:
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	// Kembalikan galat pertama (bila ada) setelah semua penerima diproses.
	return firstErr
}
