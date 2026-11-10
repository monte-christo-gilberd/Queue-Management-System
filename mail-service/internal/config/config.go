package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// MinTokenLen: panjang minimum token layanan, sama dengan yang dipakai untuk QR_SERVICE_TOKEN.
const MinTokenLen = 32

// Config adalah konfigurasi runtime mail-service.
type Config struct {
	// Port: port HTTP tempat layanan mendengarkan. Bawaan 8092.
	Port string
	// ServiceToken: nilai X-Service-Token yang wajib dibawa setiap pemanggil API kirim.
	ServiceToken string
	// SMTP: pengaturan server email tujuan kirim.
	SMTP SMTP
	// Outbox: pengaturan worker yang mengirim email dari tabel notification_outbox.
	Outbox Outbox
}

// Outbox: pengiriman email dari tabel notification_outbox. Tanpa DATABASE_URL worker mati dan layanan hanya
// melayani POST /internal/v1/send.
type Outbox struct {
	// DatabaseURL: URL PostgreSQL untuk outbox (DATABASE_URL). Kosong = worker mati.
	DatabaseURL string
	// PublicOrigin: alamat dasar aplikasi (QMS_PUBLIC_ORIGIN), dipakai membuat tautan di email. Tanpa garis miring di akhir.
	PublicOrigin string
	// Rate: jumlah email per detik (OUTBOX_RATE). Bawaan 5, batas (0, 100].
	Rate float64
	// MaxAttempts: jumlah percobaan maksimal sebelum baris ditandai gagal permanen (OUTBOX_MAX_ATTEMPTS). Bawaan 8, batas [1, 50].
	MaxAttempts int
}

// Enabled: worker outbox hanya jalan bila database dikonfigurasi.
func (o Outbox) Enabled() bool { return o.DatabaseURL != "" }

// Load gagal cepat bila token layanan atau SMTP tidak lengkap atau tidak masuk akal.
func Load() (Config, error) {
	// Baca PORT dan MAIL_SERVICE_TOKEN dari environment.
	cfg := Config{
		Port:         strings.TrimSpace(os.Getenv("PORT")),
		ServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
	}
	// PORT kosong: pakai bawaan 8092.
	if cfg.Port == "" {
		cfg.Port = "8092"
	}
	// Token terlalu pendek atau kosong: tolak, karena API kirim tidak bisa diamankan.
	if len(cfg.ServiceToken) < MinTokenLen {
		return Config{}, fmt.Errorf("MAIL_SERVICE_TOKEN must be set and at least %d characters", MinTokenLen)
	}
	// Baca dan validasi pengaturan SMTP.
	smtp, err := LoadSMTP()
	if err != nil {
		return Config{}, err
	}
	cfg.SMTP = smtp
	// Baca dan validasi pengaturan outbox.
	ob, err := loadOutbox()
	if err != nil {
		return Config{}, err
	}
	cfg.Outbox = ob
	return cfg, nil
}

// loadOutbox membaca pengaturan outbox dan memeriksa nilainya. Tautan publik wajib absolut bila outbox aktif.
func loadOutbox() (Outbox, error) {
	// Mulai dari bawaan: laju 5 email per detik dan 8 percobaan.
	ob := Outbox{
		DatabaseURL:  strings.TrimSpace(os.Getenv("DATABASE_URL")),
		PublicOrigin: strings.TrimRight(strings.TrimSpace(os.Getenv("QMS_PUBLIC_ORIGIN")), "/"),
		Rate:         5,
		MaxAttempts:  8,
	}
	// OUTBOX_RATE diisi: ubah ke angka dan pastikan berada di (0, 100].
	if v := strings.TrimSpace(os.Getenv("OUTBOX_RATE")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 100 {
			return Outbox{}, fmt.Errorf("OUTBOX_RATE must be a number in (0, 100], got %q", v)
		}
		ob.Rate = f
	}
	// OUTBOX_MAX_ATTEMPTS diisi: ubah ke bilangan bulat dan pastikan berada di [1, 50].
	if v := strings.TrimSpace(os.Getenv("OUTBOX_MAX_ATTEMPTS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			return Outbox{}, fmt.Errorf("OUTBOX_MAX_ATTEMPTS must be an integer in [1, 50], got %q", v)
		}
		ob.MaxAttempts = n
	}
	// Outbox tidak aktif (DATABASE_URL kosong): tidak perlu memeriksa tautan publik.
	if !ob.Enabled() {
		return ob, nil
	}
	// Tautan di email harus absolut; tanpa itu pelanggan menerima tautan rusak.
	u, err := url.Parse(ob.PublicOrigin)
	// Hanya http atau https dengan host yang boleh dipakai; selain itu tolak.
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Outbox{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be an absolute http(s) URL when DATABASE_URL is set, got %q", ob.PublicOrigin)
	}
	return ob, nil
}
