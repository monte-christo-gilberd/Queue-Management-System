// Package outbox: worker yang mengirim email dari tabel notification_outbox. Trigger database menulis baris
// dalam transaksi yang sama dengan perubahan tiket, jadi email tidak hilang walau layanan ini mati; baris
// menunggu sampai worker hidup lagi. Pengiriman at-least-once: bila proses mati tepat setelah SMTP menerima
// pesan dan sebelum baris ditandai terkirim, email bisa terkirim dua kali setelah masa sewa (lease) habis.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"qms/mail-service/internal/mailer"
)

// Item: satu email yang diklaim. Attempts sudah termasuk percobaan ini.
type Item struct {
	ID       int64
	Kind     string
	Email    string
	Attempts int
	Payload  json.RawMessage
}

// Store: akses ke outbox (pgx di produksi, tiruan di tes).
type Store interface {
	// Claim mengklaim paling banyak limit baris yang jatuh tempo, menaikkan attempts, dan menyewa baris
	// sebesar lease supaya worker lain (atau proses ini setelah crash) tidak mengambilnya bersamaan.
	Claim(ctx context.Context, limit int, lease time.Duration) ([]Item, error)
	MarkSent(ctx context.Context, id int64) error
	MarkRetry(ctx context.Context, id int64, cause string, delay time.Duration) error
	MarkFailed(ctx context.Context, id int64, cause string) error
}

// Sender: mailer.Mailer (tes memakai tiruan).
type Sender interface {
	Send(ctx context.Context, msg mailer.Message) error
}

type Config struct {
	PublicOrigin string        // dasar tautan di email, mis. http://localhost:8080
	Rate         float64       // email per detik
	MaxAttempts  int           // setelah ini baris ditandai gagal permanen
	Batch        int           // baris per klaim
	Poll         time.Duration // jaring pengaman bila NOTIFY terlewat
	SendTimeout  time.Duration
	BackoffBase  time.Duration // jeda sebelum percobaan ke-2; berlipat tiap gagal
	BackoffMax   time.Duration
}

// Defaults mengisi nilai yang belum diatur.
func (c Config) withDefaults() Config {
	if c.Rate <= 0 {
		c.Rate = 5
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 8
	}
	if c.Batch <= 0 {
		c.Batch = 20
	}
	if c.Poll <= 0 {
		c.Poll = 5 * time.Second
	}
	if c.SendTimeout <= 0 {
		c.SendTimeout = 45 * time.Second
	}
	if c.BackoffBase <= 0 {
		c.BackoffBase = 30 * time.Second
	}
	if c.BackoffMax <= 0 {
		c.BackoffMax = 30 * time.Minute
	}
	return c
}

type Worker struct {
	store  Store
	sender Sender
	cfg    Config
	log    *slog.Logger
	wake   chan struct{}
	now    func() time.Time
}

func NewWorker(store Store, sender Sender, cfg Config, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{store: store, sender: sender, cfg: cfg.withDefaults(), log: log, wake: make(chan struct{}, 1), now: time.Now}
}

// Wake membangunkan worker (dipanggil saat NOTIFY tiba). Tidak memblokir; sinyal ganda digabung.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run berjalan sampai ctx selesai. Kegagalan database tidak menghentikannya: dicoba lagi pada siklus berikut.
func (w *Worker) Run(ctx context.Context) {
	tick := time.NewTicker(w.cfg.Poll)
	defer tick.Stop()
	interval := time.Duration(float64(time.Second) / w.cfg.Rate)
	limiter := time.NewTicker(interval)
	defer limiter.Stop()
	for {
		w.drain(ctx, limiter.C)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-tick.C:
		}
	}
}

// drain mengirim semua baris yang jatuh tempo, satu per satu sesuai laju.
func (w *Worker) drain(ctx context.Context, limiter <-chan time.Time) {
	for ctx.Err() == nil {
		items, err := w.store.Claim(ctx, w.cfg.Batch, w.cfg.SendTimeout+time.Minute)
		if err != nil {
			if ctx.Err() == nil {
				w.log.Error("outbox claim failed", "err", err)
			}
			return
		}
		if len(items) == 0 {
			return
		}
		for _, it := range items {
			select {
			case <-ctx.Done():
				return
			case <-limiter:
			}
			w.deliver(ctx, it)
		}
	}
}

func (w *Worker) deliver(ctx context.Context, it Item) {
	msg, err := w.compose(it)
	if err != nil {
		// Templat atau payload rusak tidak akan sembuh dengan diulang.
		w.fail(ctx, it, "compose: "+err.Error())
		return
	}
	sctx, cancel := context.WithTimeout(ctx, w.cfg.SendTimeout)
	err = w.sender.Send(sctx, msg)
	cancel()
	switch {
	case err == nil:
		if e := w.store.MarkSent(ctx, it.ID); e != nil {
			w.log.Error("mark sent failed; email may be resent after lease", "id", it.ID, "err", e)
			return
		}
		w.log.Info("email sent", "id", it.ID, "kind", it.Kind)
	case ctx.Err() != nil:
		// Proses berhenti: lepaskan segera tanpa menghitung sebagai kegagalan; lease yang menjaga.
	case errors.Is(err, mailer.ErrPermanent):
		w.fail(ctx, it, err.Error())
	case it.Attempts >= w.cfg.MaxAttempts:
		w.fail(ctx, it, fmt.Sprintf("gave up after %d attempts: %v", it.Attempts, err))
	default:
		d := w.backoff(it.Attempts)
		w.log.Warn("email send failed, will retry", "id", it.ID, "attempt", it.Attempts, "retry_in", d.String(), "err", err)
		if e := w.store.MarkRetry(ctx, it.ID, err.Error(), d); e != nil {
			w.log.Error("mark retry failed", "id", it.ID, "err", e)
		}
	}
}

func (w *Worker) fail(ctx context.Context, it Item, cause string) {
	w.log.Error("email failed permanently", "id", it.ID, "kind", it.Kind, "cause", cause)
	if e := w.store.MarkFailed(ctx, it.ID, cause); e != nil {
		w.log.Error("mark failed failed", "id", it.ID, "err", e)
	}
}

// backoff: BackoffBase * 2^(attempts-1), dibatasi BackoffMax.
func (w *Worker) backoff(attempts int) time.Duration {
	d := w.cfg.BackoffBase
	for i := 1; i < attempts && d < w.cfg.BackoffMax; i++ {
		d *= 2
	}
	if d > w.cfg.BackoffMax {
		d = w.cfg.BackoffMax
	}
	return d
}

type payload struct {
	QueueID   int64  `json:"queue_id"`
	QueueName string `json:"queue_name"`
	Prefix    string `json:"prefix"`
	Number    int    `json:"number"`
	Name      string `json:"name"`
}

// Label: format nomor tiket, sama dengan notify-service dan frontend ("A-001"; prefix kosong dianggap "A").
func Label(prefix string, number int) string {
	if prefix == "" {
		prefix = "A"
	}
	return fmt.Sprintf("%s-%03d", prefix, number)
}

func (w *Worker) compose(it Item) (mailer.Message, error) {
	if !mailer.Has(it.Kind) {
		return mailer.Message{}, fmt.Errorf("no template for kind %q", it.Kind)
	}
	var p payload
	if err := json.Unmarshal(it.Payload, &p); err != nil {
		return mailer.Message{}, fmt.Errorf("payload: %w", err)
	}
	if p.QueueID == 0 || strings.TrimSpace(p.QueueName) == "" {
		return mailer.Message{}, errors.New("payload incomplete")
	}
	if strings.TrimSpace(it.Email) == "" {
		return mailer.Message{}, errors.New("recipient has no email")
	}
	link := strings.TrimRight(w.cfg.PublicOrigin, "/") + "/ticket/" + url.PathEscape(fmt.Sprint(p.QueueID))
	r, err := mailer.Render(it.Kind, map[string]any{
		"Name": p.Name, "Queue": p.QueueName, "Label": Label(p.Prefix, p.Number), "Link": link,
	})
	if err != nil {
		return mailer.Message{}, err
	}
	return mailer.Message{To: it.Email, Subject: r.Subject, Text: r.Text, HTML: r.HTML}, nil
}
