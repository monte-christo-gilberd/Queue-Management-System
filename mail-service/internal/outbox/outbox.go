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

// Item: satu email yang diklaim dari outbox. Attempts sudah termasuk percobaan ini.
type Item struct {
	// ID: id baris outbox.
	ID int64
	// Kind: jenis email, sekaligus nama templatnya (mis. ticket-called).
	Kind string
	// Email: alamat penerima, diambil dari tabel users.
	Email string
	// Attempts: jumlah percobaan termasuk percobaan ini.
	Attempts int
	// Payload: data JSON dari baris outbox (nomor, nama antrian, dll.).
	Payload json.RawMessage
}

// Store: akses ke outbox (pgx di produksi, tiruan di tes).
type Store interface {
	// Claim mengklaim paling banyak limit baris yang jatuh tempo, menaikkan attempts, dan menyewa baris
	// sebesar lease supaya worker lain (atau proses ini setelah crash) tidak mengambilnya bersamaan.
	Claim(ctx context.Context, limit int, lease time.Duration) ([]Item, error)
	// MarkSent menandai baris sudah terkirim.
	MarkSent(ctx context.Context, id int64) error
	// MarkRetry menjadwalkan percobaan ulang setelah delay dan menyimpan penyebab galat terakhir.
	MarkRetry(ctx context.Context, id int64, cause string, delay time.Duration) error
	// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya.
	MarkFailed(ctx context.Context, id int64, cause string) error
}

// Sender: mailer.Mailer (tes memakai tiruan).
type Sender interface {
	// Send mengirim satu pesan email.
	Send(ctx context.Context, msg mailer.Message) error
}

// Config: pengaturan worker. Nilai nol memakai bawaan (lihat withDefaults).
type Config struct {
	// PublicOrigin: dasar tautan di email, mis. http://localhost:8080.
	PublicOrigin string
	// Rate: jumlah email per detik.
	Rate float64
	// MaxAttempts: setelah jumlah ini, baris ditandai gagal permanen.
	MaxAttempts int
	// Batch: jumlah baris per klaim.
	Batch int
	// Poll: jeda polling, jaring pengaman bila NOTIFY terlewat.
	Poll time.Duration
	// SendTimeout: batas waktu satu pengiriman.
	SendTimeout time.Duration
	// BackoffBase: jeda sebelum percobaan ke-2; berlipat tiap gagal.
	BackoffBase time.Duration
	// BackoffMax: batas atas jeda antar percobaan.
	BackoffMax time.Duration
}

// withDefaults mengisi nilai yang belum diatur dengan bawaan: 5 email per detik, 8 percobaan, batch 20,
// polling 5 detik, batas kirim 45 detik, backoff mulai 30 detik dan maksimal 30 menit.
func (c Config) withDefaults() Config {
	// Laju tidak diisi (nol atau negatif): 5 email per detik.
	if c.Rate <= 0 {
		c.Rate = 5
	}
	// Batas percobaan tidak diisi: 8.
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 8
	}
	// Ukuran batch tidak diisi: 20 baris.
	if c.Batch <= 0 {
		c.Batch = 20
	}
	// Jeda polling tidak diisi: 5 detik.
	if c.Poll <= 0 {
		c.Poll = 5 * time.Second
	}
	// Batas waktu kirim tidak diisi: 45 detik.
	if c.SendTimeout <= 0 {
		c.SendTimeout = 45 * time.Second
	}
	// Backoff awal tidak diisi: 30 detik.
	if c.BackoffBase <= 0 {
		c.BackoffBase = 30 * time.Second
	}
	// Batas backoff tidak diisi: 30 menit.
	if c.BackoffMax <= 0 {
		c.BackoffMax = 30 * time.Minute
	}
	return c
}

// Worker: loop yang mengklaim baris outbox, mengirim email sesuai laju, dan mencatat hasilnya.
type Worker struct {
	// store: akses ke tabel outbox.
	store Store
	// sender: pengirim email.
	sender Sender
	// cfg: pengaturan setelah diisi nilai bawaan.
	cfg Config
	// log: pencatat peristiwa worker.
	log *slog.Logger
	// wake: sinyal untuk membangunkan worker sebelum polling berikutnya. Berkapasitas 1 supaya sinyal ganda digabung.
	wake chan struct{}
	// now: sumber waktu. Bisa diganti di tes.
	now func() time.Time
}

// NewWorker membuat Worker. Nilai cfg yang kosong diisi bawaan, dan logger nil memakai slog default.
func NewWorker(store Store, sender Sender, cfg Config, log *slog.Logger) *Worker {
	// Logger nil: pakai slog default.
	if log == nil {
		log = slog.Default()
	}
	// Susun Worker dengan cfg yang sudah diisi bawaan dan saluran wake berkapasitas 1.
	return &Worker{store: store, sender: sender, cfg: cfg.withDefaults(), log: log, wake: make(chan struct{}, 1), now: time.Now}
}

// Wake membangunkan worker (dipanggil saat NOTIFY tiba). Tidak memblokir; sinyal ganda digabung.
func (w *Worker) Wake() {
	// Coba kirim sinyal. Bila saluran sudah berisi sinyal, lewati agar tidak memblokir.
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run berjalan sampai ctx selesai. Kegagalan database tidak menghentikannya: dicoba lagi pada siklus berikut.
func (w *Worker) Run(ctx context.Context) {
	// Timer polling sebagai jaring pengaman bila NOTIFY terlewat.
	tick := time.NewTicker(w.cfg.Poll)
	defer tick.Stop()
	// Jeda antar email dari laju per detik.
	interval := time.Duration(float64(time.Second) / w.cfg.Rate)
	limiter := time.NewTicker(interval)
	defer limiter.Stop()
	// Ulangi siklus: kosongkan antrian, lalu tunggu sinyal atau polling.
	for {
		// Kirim semua baris yang jatuh tempo.
		w.drain(ctx, limiter.C)
		// Tunggu: konteks batal berhenti, sinyal wake atau polling melanjutkan siklus.
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
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Klaim satu batch baris. Lease dipasang sedikit lebih lama dari batas kirim.
		items, err := w.store.Claim(ctx, w.cfg.Batch, w.cfg.SendTimeout+time.Minute)
		// Gagal klaim: catat (kecuali karena shutdown), lalu berhenti sampai siklus berikutnya.
		if err != nil {
			if ctx.Err() == nil {
				w.log.Error("outbox claim failed", "err", err)
			}
			return
		}
		// Tidak ada baris yang jatuh tempo: selesai untuk siklus ini.
		if len(items) == 0 {
			return
		}
		// Kirim setiap baris, menunggu giliran dari limiter agar laju tetap terjaga.
		for _, it := range items {
			select {
			// Shutdown: berhenti tanpa mengirim baris berikutnya.
			case <-ctx.Done():
				return
			// Giliran tiba: kirim baris ini.
			case <-limiter:
			}
			w.deliver(ctx, it)
		}
	}
}

// deliver menyusun email untuk satu baris, mengirimnya, lalu mencatat hasilnya: terkirim, dicoba lagi, atau gagal.
func (w *Worker) deliver(ctx context.Context, it Item) {
	// Susun email dari templat dan payload. Galat di sini tidak akan sembuh dengan diulang.
	msg, err := w.compose(it)
	if err != nil {
		// Templat atau payload rusak tidak akan sembuh dengan diulang.
		w.fail(ctx, it, "compose: "+err.Error())
		return
	}
	// Batasi satu pengiriman dengan SendTimeout.
	sctx, cancel := context.WithTimeout(ctx, w.cfg.SendTimeout)
	err = w.sender.Send(sctx, msg)
	cancel()
	// Pilih tindakan sesuai hasil kirim.
	switch {
	// Terkirim: tandai terkirim dan catat.
	case err == nil:
		// Gagal menandai terkirim: catat, dan biarkan lease habis lalu email mungkin dikirim ulang.
		if e := w.store.MarkSent(ctx, it.ID); e != nil {
			w.log.Error("mark sent failed; email may be resent after lease", "id", it.ID, "err", e)
			return
		}
		w.log.Info("email sent", "id", it.ID, "kind", it.Kind)
	// Proses berhenti: lepaskan segera tanpa menghitung sebagai kegagalan; lease yang menjaga.
	case ctx.Err() != nil:
	// Galat permanen dari SMTP: gagal permanen, tidak perlu dicoba lagi.
	case errors.Is(err, mailer.ErrPermanent):
		w.fail(ctx, it, err.Error())
	// Sudah mencapai batas percobaan: gagal permanen.
	case it.Attempts >= w.cfg.MaxAttempts:
		w.fail(ctx, it, fmt.Sprintf("gave up after %d attempts: %v", it.Attempts, err))
	// Galat sementara dan masih ada kesempatan: jadwalkan percobaan ulang dengan backoff.
	default:
		// Hitung jeda sesuai jumlah percobaan sejauh ini.
		d := w.backoff(it.Attempts)
		w.log.Warn("email send failed, will retry", "id", it.ID, "attempt", it.Attempts, "retry_in", d.String(), "err", err)
		// Simpan jadwal percobaan ulang. Gagal menyimpan dicatat saja.
		if e := w.store.MarkRetry(ctx, it.ID, err.Error(), d); e != nil {
			w.log.Error("mark retry failed", "id", it.ID, "err", e)
		}
	}
}

// fail menandai baris gagal permanen dan mencatat penyebabnya.
func (w *Worker) fail(ctx context.Context, it Item, cause string) {
	// Catat kegagalan permanen beserta jenis dan penyebabnya.
	w.log.Error("email failed permanently", "id", it.ID, "kind", it.Kind, "cause", cause)
	// Simpan status gagal ke database. Gagal menyimpan hanya dicatat.
	if e := w.store.MarkFailed(ctx, it.ID, cause); e != nil {
		w.log.Error("mark failed failed", "id", it.ID, "err", e)
	}
}

// backoff menghitung jeda sebelum percobaan berikutnya: BackoffBase * 2^(attempts-1), dibatasi BackoffMax.
func (w *Worker) backoff(attempts int) time.Duration {
	// Mulai dari jeda dasar.
	d := w.cfg.BackoffBase
	// Gandakan jeda untuk setiap percobaan tambahan, selama belum mencapai batas maksimal.
	for i := 1; i < attempts && d < w.cfg.BackoffMax; i++ {
		d *= 2
	}
	// Jeda melewati batas maksimal: potong ke batas.
	if d > w.cfg.BackoffMax {
		d = w.cfg.BackoffMax
	}
	return d
}

// payload: data yang dibaca dari kolom payload baris outbox. Dibentuk oleh trigger database.
type payload struct {
	// QueueID: id antrian. Wajib tidak nol.
	QueueID int64 `json:"queue_id"`
	// QueueName: nama antrian. Wajib tidak kosong.
	QueueName string `json:"queue_name"`
	// Prefix: prefix nomor tiket.
	Prefix string `json:"prefix"`
	// Number: nomor tiket tanpa prefix.
	Number int `json:"number"`
	// Name: nama penerima untuk sapaan di email.
	Name string `json:"name"`
}

// Label membuat label nomor tiket untuk email, mis. "A-001". Prefix kosong dianggap "A".
// Sama dengan notify-service dan frontend.
func Label(prefix string, number int) string {
	// Prefix kosong: pakai "A".
	if prefix == "" {
		prefix = "A"
	}
	// Gabungkan prefix dan nomor dengan tiga digit.
	return fmt.Sprintf("%s-%03d", prefix, number)
}

// compose menyusun mailer.Message dari baris outbox: memeriksa jenis, payload, dan alamat, lalu mengisi templat.
func (w *Worker) compose(it Item) (mailer.Message, error) {
	// Templat untuk jenis ini tidak ada: tidak bisa disusun.
	if !mailer.Has(it.Kind) {
		return mailer.Message{}, fmt.Errorf("no template for kind %q", it.Kind)
	}
	// Baca payload JSON. Gagal dibaca: payload rusak.
	var p payload
	if err := json.Unmarshal(it.Payload, &p); err != nil {
		return mailer.Message{}, fmt.Errorf("payload: %w", err)
	}
	// Id antrian atau nama antrian kosong: payload tidak lengkap.
	if p.QueueID == 0 || strings.TrimSpace(p.QueueName) == "" {
		return mailer.Message{}, errors.New("payload incomplete")
	}
	// Penerima tidak punya alamat email: tidak bisa dikirim.
	if strings.TrimSpace(it.Email) == "" {
		return mailer.Message{}, errors.New("recipient has no email")
	}
	// Bentuk tautan ke halaman antrian, dengan id antrian yang sudah di-escape.
	link := strings.TrimRight(w.cfg.PublicOrigin, "/") + "/ticket/" + url.PathEscape(fmt.Sprint(p.QueueID))
	// Isi templat dengan nama, antrian, label nomor, dan tautan.
	r, err := mailer.Render(it.Kind, map[string]any{
		"Name": p.Name, "Queue": p.QueueName, "Label": Label(p.Prefix, p.Number), "Link": link,
	})
	if err != nil {
		return mailer.Message{}, err
	}
	// Kembalikan pesan siap kirim.
	return mailer.Message{To: it.Email, Subject: r.Subject, Text: r.Text, HTML: r.HTML}, nil
}
