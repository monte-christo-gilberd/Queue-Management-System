package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/model"
)

// OutboxStore: akses ke outbox (dipenuhi repository.OutboxRepository di produksi, tiruan di tes).
type OutboxStore interface {
	// Claim mengklaim paling banyak limit baris yang jatuh tempo, menaikkan attempts, dan menyewa baris
	// sebesar lease supaya worker lain (atau proses ini setelah crash) tidak mengambilnya bersamaan.
	Claim(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]model.OutboxItem, error)
	// MarkSent menandai baris sudah terkirim.
	MarkSent(
		ctx context.Context,
		id int64,
	) error
	// MarkRetry menjadwalkan percobaan ulang setelah delay dan menyimpan penyebab galat terakhir.
	MarkRetry(
		ctx context.Context,
		id int64,
		cause string,
		delay time.Duration,
	) error
	// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya.
	MarkFailed(
		ctx context.Context,
		id int64,
		cause string,
	) error
}

// OutboxConfig: pengaturan worker. Nilai nol memakai bawaan (lihat withDefaults).
type OutboxConfig struct {
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
func (c OutboxConfig) withDefaults() OutboxConfig {
	// Laju tidak diisi (nol atau negatif): 5 email per detik.
	if c.Rate <= 0 {
		// Ukuran 5 per detik menjaga SMTP tidak kebanjiran saat banyak antrian dipanggil sekaligus.
		c.Rate = 5
	}
	// Batas percobaan tidak diisi: 8.
	if c.MaxAttempts <= 0 {
		// Delapan percobaan dengan backoff berlipat kira-kira menutup gangguan SMTP beberapa jam.
		c.MaxAttempts = 8
	}
	// Ukuran batch tidak diisi: 20 baris.
	if c.Batch <= 0 {
		// Batch kecil membuat lease satu klaim tetap masuk akal terhadap laju kirim.
		c.Batch = 20
	}
	// Jeda polling tidak diisi: 5 detik.
	if c.Poll <= 0 {
		// Polling hanya cadangan, jadi 5 detik cukup rapat tanpa membebani database.
		c.Poll = 5 * time.Second
	}
	// Batas waktu kirim tidak diisi: 45 detik.
	if c.SendTimeout <= 0 {
		// Batas ini lebih besar dari batas satu percakapan SMTP (3 percobaan di dalam Mailer).
		c.SendTimeout = 45 * time.Second
	}
	// Backoff awal tidak diisi: 30 detik.
	if c.BackoffBase <= 0 {
		// Awal 30 detik memberi SMTP waktu pulih sebelum dicoba lagi.
		c.BackoffBase = 30 * time.Second
	}
	// Batas backoff tidak diisi: 30 menit.
	if c.BackoffMax <= 0 {
		// Batas 30 menit mencegah jeda tumbuh tanpa batas.
		c.BackoffMax = 30 * time.Minute
	}
	// Kembalikan salinan yang sudah terisi.
	return c
}

// OutboxService: loop yang mengklaim baris outbox, mengirim email sesuai laju, dan mencatat hasilnya.
type OutboxService struct {
	// store: akses ke tabel outbox.
	store OutboxStore
	// sender: pengirim email.
	sender Sender
	// renderer: pengisi templat email.
	renderer TemplateRenderer
	// cfg: pengaturan setelah diisi nilai bawaan.
	cfg OutboxConfig
	// log: pencatat peristiwa worker.
	log *slog.Logger
	// wake: sinyal untuk membangunkan worker sebelum polling berikutnya. Berkapasitas 1 supaya sinyal ganda digabung.
	wake chan struct{}
	// now: sumber waktu. Bisa diganti di tes.
	now func() time.Time
}

// NewOutboxService membuat OutboxService. Nilai cfg yang kosong diisi bawaan, dan logger nil memakai slog default.
func NewOutboxService(
	store OutboxStore,
	sender Sender,
	renderer TemplateRenderer,
	cfg OutboxConfig,
	log *slog.Logger,
) *OutboxService {
	// Logger nil: pakai slog default.
	if log == nil {
		// Pakai logger bawaan agar pemanggil tidak wajib menyiapkannya.
		log = slog.Default()
	}
	// Susun Worker dengan cfg yang sudah diisi bawaan dan saluran wake berkapasitas 1.
	return &OutboxService{store: store, sender: sender, renderer: renderer, cfg: cfg.withDefaults(), log: log, wake: make(chan struct{}, 1), now: time.Now}
}

// Wake membangunkan worker (dipanggil saat NOTIFY tiba). Tidak memblokir; sinyal ganda digabung.
func (w *OutboxService) Wake() {
	// Coba kirim sinyal. Bila saluran sudah berisi sinyal, lewati agar tidak memblokir.
	select {
	// Saluran kosong: sinyal masuk.
	case w.wake <- struct{}{}:
	// Saluran sudah berisi sinyal: lewati, karena worker akan tetap bangun.
	default:
	}
}

// Run berjalan sampai ctx selesai. Kegagalan database tidak menghentikannya: dicoba lagi pada siklus berikut.
func (w *OutboxService) Run(
	ctx context.Context,
) {
	// Timer polling sebagai jaring pengaman bila NOTIFY terlewat.
	tick := time.NewTicker(w.cfg.Poll)
	// Hentikan ticker saat Run selesai.
	defer tick.Stop()
	// Jeda antar email dari laju per detik.
	interval := time.Duration(float64(time.Second) / w.cfg.Rate)
	// Limiter berdetak sekali per interval; satu detak adalah izin kirim satu email.
	limiter := time.NewTicker(interval)
	// Hentikan limiter saat Run selesai.
	defer limiter.Stop()
	// Ulangi siklus: kosongkan antrian, lalu tunggu sinyal atau polling.
	for {
		// Kirim semua baris yang jatuh tempo.
		w.drain(ctx, limiter.C)
		// Tunggu: konteks batal berhenti, sinyal wake atau polling melanjutkan siklus.
		select {
		// Konteks batal: keluar dari Run.
		case <-ctx.Done():
			// Shutdown bersih: baris yang belum diproses tetap di database.
			return
		// Ada sinyal NOTIFY: langsung kuras lagi.
		case <-w.wake:
		// Polling berdetak: kuras lagi walau tidak ada sinyal, kalau NOTIFY terlewat.
		case <-tick.C:
		}
	}
}

// drain mengirim semua baris yang jatuh tempo, satu per satu sesuai laju.
func (w *OutboxService) drain(
	ctx context.Context,
	limiter <-chan time.Time,
) {
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Klaim satu batch baris. Lease dipasang sedikit lebih lama dari batas kirim.
		items, err := w.store.Claim(ctx, w.cfg.Batch, w.cfg.SendTimeout+time.Minute)
		// Gagal klaim: catat (kecuali karena shutdown), lalu berhenti sampai siklus berikutnya.
		if err != nil {
			// Kegagalan karena shutdown bukan kesalahan, jadi tidak dicatat.
			if ctx.Err() == nil {
				// Catat agar masalah database terlihat.
				w.log.Error("outbox claim failed", "err", err)
			}
			// Dicoba lagi pada siklus berikut.
			return
		}
		// Tidak ada baris yang jatuh tempo: selesai untuk siklus ini.
		if len(items) == 0 {
			// Antrian kosong: tunggu sinyal atau polling.
			return
		}
		// Kirim setiap baris, menunggu giliran dari limiter agar laju tetap terjaga.
		for _, it := range items {
			// Tunggu izin dari limiter, atau berhenti bila shutdown.
			select {
			// Shutdown: berhenti tanpa mengirim baris berikutnya.
			case <-ctx.Done():
				// Shutdown: tinggalkan sisa batch; lease akan habis dan baris diklaim ulang.
				return
			// Giliran tiba: kirim baris ini.
			case <-limiter:
			}
			// Kirim baris ini dan catat hasilnya.
			w.deliver(ctx, it)
		}
	}
}

// deliver menyusun email untuk satu baris, mengirimnya, lalu mencatat hasilnya: terkirim, dicoba lagi, atau gagal.
func (w *OutboxService) deliver(
	ctx context.Context,
	it model.OutboxItem,
) {
	// Susun email dari templat dan payload. Galat di sini tidak akan sembuh dengan diulang.
	msg, err := w.compose(it)
	// Gagal menyusun email: tandai gagal permanen dan berhenti memproses baris ini.
	if err != nil {
		// Templat atau payload rusak tidak akan sembuh dengan diulang.
		w.fail(ctx, it, "compose: "+err.Error())
		return
	}
	// Batasi satu pengiriman dengan SendTimeout.
	sctx, cancel := context.WithTimeout(ctx, w.cfg.SendTimeout)
	// Kirim lewat sender dengan konteks bertenggat.
	err = w.sender.Send(sctx, msg)
	// Lepas timer konteks segera, tidak menunggu fungsi selesai.
	cancel()
	// Pilih tindakan sesuai hasil kirim.
	switch {
	// Terkirim: tandai terkirim dan catat.
	case err == nil:
		// Gagal menandai terkirim: catat, dan biarkan lease habis lalu email mungkin dikirim ulang.
		if e := w.store.MarkSent(ctx, it.ID); e != nil {
			w.log.Error("mark sent failed; email may be resent after lease", "id", it.ID, "err", e)
			// Tanpa MarkSent baris akan diklaim lagi setelah lease habis, jadi pengiriman ulang tidak bisa dihindari.
			return
		}
		w.log.Info("email sent", "id", it.ID, "kind", it.Kind)
	// Proses berhenti: lepaskan segera tanpa menghitung sebagai kegagalan; lease yang menjaga.
	case ctx.Err() != nil:
	// Galat permanen dari SMTP: gagal permanen, tidak perlu dicoba lagi.
	case errors.Is(err, apperror.ErrPermanent):
		// Simpan galat permanen sebagai penyebab terakhir.
		w.fail(ctx, it, err.Error())
	// Sudah mencapai batas percobaan: gagal permanen.
	case it.Attempts >= w.cfg.MaxAttempts:
		// Penyebab mencantumkan jumlah percobaan agar mudah dilacak.
		w.fail(ctx, it, fmt.Sprintf("gave up after %d attempts: %v", it.Attempts, err))
	// Galat sementara dan masih ada kesempatan: jadwalkan percobaan ulang dengan backoff.
	default:
		// Hitung jeda sesuai jumlah percobaan sejauh ini.
		d := w.backoff(it.Attempts)
		// Catat percobaan ulang beserta jeda dan penyebabnya.
		w.log.Warn("email send failed, will retry", "id", it.ID, "attempt", it.Attempts, "retry_in", d.String(), "err", err)
		// Simpan jadwal percobaan ulang. Gagal menyimpan dicatat saja.
		if e := w.store.MarkRetry(ctx, it.ID, err.Error(), d); e != nil {
			// Baris tetap terlepas setelah lease habis, jadi tetap dicoba ulang.
			w.log.Error("mark retry failed", "id", it.ID, "err", e)
		}
	}
}

// fail menandai baris gagal permanen dan mencatat penyebabnya.
func (w *OutboxService) fail(
	ctx context.Context,
	it model.OutboxItem,
	cause string,
) {
	// Catat kegagalan permanen beserta jenis dan penyebabnya.
	w.log.Error("email failed permanently", "id", it.ID, "kind", it.Kind, "cause", cause)
	// Simpan status gagal ke database. Gagal menyimpan hanya dicatat.
	if e := w.store.MarkFailed(ctx, it.ID, cause); e != nil {
		// Gagal menyimpan: baris akan diklaim lagi setelah lease habis.
		w.log.Error("mark failed failed", "id", it.ID, "err", e)
	}
}

// backoff menghitung jeda sebelum percobaan berikutnya: BackoffBase * 2^(attempts-1), dibatasi BackoffMax.
func (w *OutboxService) backoff(
	attempts int,
) time.Duration {
	// Mulai dari jeda dasar.
	d := w.cfg.BackoffBase
	// Gandakan jeda untuk setiap percobaan tambahan, selama belum mencapai batas maksimal.
	for i := 1; i < attempts && d < w.cfg.BackoffMax; i++ {
		// Gandakan jeda (backoff eksponensial).
		d *= 2
	}
	// Jeda melewati batas maksimal: potong ke batas.
	if d > w.cfg.BackoffMax {
		// Potong ke batas maksimal.
		d = w.cfg.BackoffMax
	}
	// Kembalikan jeda akhir.
	return d
}

// Label membuat label nomor tiket untuk email, mis. "A-001". Prefix kosong dianggap "A".
// Sama dengan notify-service dan frontend.
func Label(
	prefix string,
	number int,
) string {
	// Prefix kosong: pakai "A".
	if prefix == "" {
		// Label tiket memakai awalan A bila tidak diisi.
		prefix = "A"
	}
	// Gabungkan prefix dan nomor dengan tiga digit.
	return fmt.Sprintf("%s-%03d", prefix, number)
}

// compose menyusun model.Message dari baris outbox: memeriksa jenis, payload, dan alamat, lalu mengisi templat.
func (w *OutboxService) compose(
	it model.OutboxItem,
) (model.Message, error) {
	// Templat untuk jenis ini tidak ada: tidak bisa disusun.
	if !w.renderer.Has(it.Kind) {
		// Jenis email tidak dikenal.
		return model.Message{}, fmt.Errorf("no template for kind %q", it.Kind)
	}
	// Baca payload JSON. Gagal dibaca: payload rusak.
	var p model.OutboxPayload
	// Payload bukan JSON yang sesuai.
	if err := json.Unmarshal(it.Payload, &p); err != nil {
		return model.Message{}, fmt.Errorf("payload: %w", err)
	}
	// Id antrian atau nama antrian kosong: payload tidak lengkap.
	if p.QueueID == 0 || strings.TrimSpace(p.QueueName) == "" {
		// Tanpa id atau nama antrian, templat tidak bisa diisi.
		return model.Message{}, errors.New("payload incomplete")
	}
	// Penerima tidak punya alamat email: tidak bisa dikirim.
	if strings.TrimSpace(it.Email) == "" {
		// Tanpa alamat, tidak ada tujuan kirim.
		return model.Message{}, errors.New("recipient has no email")
	}
	// Bentuk tautan ke halaman antrian, dengan id antrian yang sudah di-escape.
	link := strings.TrimRight(w.cfg.PublicOrigin, "/") + "/ticket/" + url.PathEscape(fmt.Sprint(p.QueueID))
	// Antrian ditutup owner: tautan ke riwayat owner, tempat antrian yang sudah ditutup berada.
	if it.Kind == "queue-closed" {
		link = strings.TrimRight(w.cfg.PublicOrigin, "/") + "/riwayat"
	}
	// Isi templat dengan nama, antrian, label nomor, dan tautan.
	r, err := w.renderer.Render(it.Kind, map[string]any{
		"Name": p.Name, "Queue": p.QueueName, "Label": Label(p.Prefix, p.Number), "Link": link,
	})
	// Render gagal: kembalikan galat templat.
	if err != nil {
		return model.Message{}, err
	}
	// Kembalikan pesan siap kirim.
	return model.Message{To: it.Email, Subject: r.Subject, Text: r.Text, HTML: r.HTML}, nil
}
