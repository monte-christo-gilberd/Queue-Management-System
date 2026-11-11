package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"time"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/model"
)

const (
	// maxInFlight membatasi pengiriman yang berjalan bersamaan; lebih dari itu ditolak dengan ErrBusy supaya
	// pemanggil yang membanjiri tidak menghabiskan koneksi SMTP. Angka 16 cukup untuk beban normal
	// dan masih aman bagi batas koneksi server SMTP.
	maxInFlight = 16
	// sendTimeout: batas waktu satu pengiriman sinkron, termasuk coba ulang (45 detik). Lebih besar dari 3 percobaan Mailer yang tiap percakapannya dibatasi 15 detik.
	sendTimeout = 45 * time.Second
	// testSendTimeout: batas waktu pengiriman email uji dari perintah send-test (30 detik).
	testSendTimeout = 30 * time.Second
)

// Sender adalah yang benar-benar mengirim email (dipenuhi mailer.Mailer; tes memakai tiruan). Dipakai bersama
// oleh MailService dan OutboxService.
type Sender interface {
	// Send mengirim satu pesan email.
	Send(
		ctx context.Context,
		msg model.Message,
	) error
}

// TemplateRenderer adalah pengisi templat email (dipenuhi mailtemplate.Renderer). Dipakai bersama oleh
// MailService dan OutboxService.
type TemplateRenderer interface {
	// Has melaporkan apakah templat dengan nama itu ada dan lengkap.
	Has(
		name string,
	) bool
	// Render mengisi templat name dengan data.
	Render(
		name string,
		data map[string]any,
	) (model.Rendered, error)
}

// MailService: logika kirim email langsung (POST /internal/v1/send dan perintah send-test): memeriksa
// penerima, mengisi templat, membatasi pengiriman bersamaan, lalu mengirim. Dibuat lewat NewMailService.
type MailService struct {
	// sender: pengirim email.
	sender Sender
	// renderer: pengisi templat email.
	renderer TemplateRenderer
	// sem: semafor pembatas pengiriman bersamaan, berkapasitas maxInFlight.
	sem chan struct{}
	// log: pencatat peristiwa kirim dan galat.
	log *slog.Logger
}

// NewMailService membuat MailService dengan pengirim, pengisi templat, dan logger. Logger nil memakai slog default.
func NewMailService(
	sender Sender,
	renderer TemplateRenderer,
	log *slog.Logger,
) *MailService {
	// Pakai logger default bila tidak diisi.
	if log == nil {
		// Logger bawaan menghindari nil pointer saat dipakai.
		log = slog.Default()
	}
	// Susun service dengan semafor berkapasitas maxInFlight.
	return &MailService{
		sender:   sender,
		renderer: renderer,
		sem:      make(chan struct{}, maxInFlight),
		log:      log,
	}
}

// Send mengirim satu email secara sinkron (dengan coba ulang di dalam Mailer) dan kembali setelah selesai.
// Tidak ada antrian di memori: pemanggil yang butuh ketahanan terhadap mati mendadak memakai outbox.
// Galat yang dikembalikan: ErrInvalidRecipient, ErrUnknownTemplate, ErrBadData, ErrBusy, atau ErrSendFailed
// (membungkus penyebab asli). Galat render lain dikembalikan apa adanya dan dibalas 500 oleh handler.
func (s *MailService) Send(
	ctx context.Context,
	to string,
	template string,
	data map[string]any,
) error {
	// Alamat penerima kosong atau tidak bisa diurai sebagai email: tolak.
	if _, err := mail.ParseAddress(to); err != nil || to == "" {
		// Kembalikan ErrInvalidRecipient; handler membalas 400.
		return apperror.ErrInvalidRecipient
	}
	// Data tidak dikirim: pakai map kosong agar templat tetap bisa diisi.
	if data == nil {
		// Map nil aman dibaca, tetapi map kosong lebih jelas bagi templat.
		data = map[string]any{}
	}
	// Isi templat dengan data.
	rendered, err := s.renderer.Render(template, data)
	// Render gagal: bedakan galat input dari galat server.
	if err != nil {
		// Templat tidak dikenal atau data tidak lengkap: kembalikan apa adanya, handler yang memetakan.
		if errors.Is(err, apperror.ErrUnknownTemplate) || errors.Is(err, apperror.ErrBadData) {
			// Galat milik pemanggil (templat atau data salah), bukan kesalahan server.
			return err
		}
		// Galat render lain adalah kesalahan server: catat dan teruskan.
		s.log.Error("render", "template", template, "err", err)
		// Teruskan agar handler membalas 500.
		return err
	}

	// Ambil satu slot dari semafor. Bila penuh, langsung tolak agar pemanggil mencoba lagi nanti.
	select {
	// Slot didapat: lanjut mengirim.
	case s.sem <- struct{}{}:
		// Lepaskan slot setelah pengiriman selesai.
		defer func() { <-s.sem }()
	// Semua slot terpakai.
	default:
		// Tolak segera, bukan menunggu: antrian tunggu hanya memperbesar beban koneksi yang sudah penuh.
		return apperror.ErrBusy
	}

	// Beri pengiriman batas waktu sendTimeout, mengikuti konteks permintaan.
	sctx, cancel := context.WithTimeout(ctx, sendTimeout)
	// Batalkan konteks saat fungsi selesai agar timer dilepas.
	defer cancel()
	// Alamat penerima sengaja tidak dicatat di log (data pribadi); cukup nama templat dan hasilnya.
	msg := model.Message{To: to, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML}
	// Kirim email; sender sudah mengulang kegagalan sementara.
	if err := s.sender.Send(sctx, msg); err != nil {
		// Galat kirim: catat apakah permanen. Detail SMTP tidak dibocorkan ke pemanggil oleh handler.
		s.log.Error("send email", "template", template, "permanent", errors.Is(err, apperror.ErrPermanent), "err", err)
		// Bungkus dengan ErrSendFailed agar handler membalas 502 tanpa membocorkan detail SMTP.
		return fmt.Errorf("%w: %w", apperror.ErrSendFailed, err)
	}
	// Berhasil terkirim: catat nama templat (bukan alamat).
	s.log.Info("email sent", "template", template)
	// Email terkirim.
	return nil
}

// SendTest mengirim satu email uji (templat "test") ke alamat to, untuk perintah `mail-service send-test`.
// Tidak lewat semafor karena hanya dijalankan satu kali dari command line.
func (s *MailService) SendTest(
	ctx context.Context,
	to string,
) error {
	// Isi templat "test" dengan alamat tujuan, waktu kirim, dan tautan contoh.
	r, err := s.renderer.Render("test", map[string]any{
		"To":     to,
		"SentAt": time.Now().Format("2 Jan 2006 15:04:05 MST"),
		"Link":   "http://localhost:8080/reset-password?token=contoh-token-uji",
	})
	// Templat gagal diisi: hentikan.
	if err != nil {
		// Kembalikan galat render dengan konteks.
		return fmt.Errorf("render: %w", err)
	}
	// Beri pengiriman batas waktu 30 detik.
	sctx, cancel := context.WithTimeout(ctx, testSendTimeout)
	// Lepas timer saat fungsi selesai.
	defer cancel()
	// Kirim email uji. Galat berarti SMTP tidak bisa dipakai.
	if err := s.sender.Send(sctx, model.Message{To: to, Subject: r.Subject, Text: r.Text, HTML: r.HTML}); err != nil {
		// Kembalikan galat kirim dengan konteks.
		return fmt.Errorf("send test email: %w", err)
	}
	// Email uji terkirim.
	return nil
}
