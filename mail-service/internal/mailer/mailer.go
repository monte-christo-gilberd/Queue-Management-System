package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"qms/mail-service/internal/config"
)

// Message: satu email. Text wajib; HTML opsional (bila ada, dikirim sebagai multipart/alternative).
type Message struct {
	// To: alamat penerima.
	To string
	// Subject: subjek email. Tidak boleh berisi CR atau LF.
	Subject string
	// Text: isi versi teks biasa. Wajib tidak kosong.
	Text string
	// HTML: isi versi HTML. Opsional.
	HTML string
}

// ErrPermanent membungkus galat yang tidak ada gunanya diulang (alamat ditolak, autentikasi gagal, 5xx).
var ErrPermanent = errors.New("permanent smtp failure")

// Mailer: pengirim email lewat SMTP dengan batas waktu dan coba ulang. Dibuat lewat New.
type Mailer struct {
	// cfg: pengaturan SMTP (host, port, TLS, autentikasi, pengirim).
	cfg config.SMTP
	// dialTimeout: batas waktu membuka koneksi TCP.
	dialTimeout time.Duration
	// attemptTime: batas waktu satu percakapan SMTP lengkap.
	attemptTime time.Duration
	// attempts: jumlah percobaan maksimal untuk satu email.
	attempts int
	// backoff: jeda awal sebelum percobaan ulang, berlipat dua tiap kegagalan.
	backoff time.Duration
	// sleep: fungsi tunggu antar percobaan. Bisa diganti di tes.
	sleep func(context.Context, time.Duration) error
	// now: sumber waktu untuk header Date dan tenggat. Bisa diganti di tes.
	now func() time.Time
}

// New membuat Mailer dengan batas waktu dan coba ulang bawaan (3 percobaan, jeda awal 1 detik).
func New(cfg config.SMTP) *Mailer {
	// Susun Mailer dengan nilai bawaan: dial 5 detik, percakapan 15 detik, 3 percobaan, jeda 1 detik.
	return &Mailer{
		cfg:         cfg,
		dialTimeout: 5 * time.Second,
		attemptTime: 15 * time.Second,
		attempts:    3,
		backoff:     time.Second,
		sleep:       sleepCtx,
		now:         time.Now,
	}
}

// WithRetry mengatur jumlah percobaan dan backoff awal (berlipat dua tiap percobaan). Untuk tes.
func (m *Mailer) WithRetry(attempts int, backoff time.Duration) *Mailer {
	// Ganti jumlah percobaan dan jeda awal, lalu kembalikan Mailer yang sama agar bisa dirangkai.
	m.attempts, m.backoff = attempts, backoff
	return m
}

// SetSleep mengganti fungsi tunggu antar percobaan. Untuk tes.
func (m *Mailer) SetSleep(f func(context.Context, time.Duration) error) { m.sleep = f }

// sleepCtx menunggu d, atau berhenti lebih cepat bila ctx batal (dan mengembalikan galat konteks).
func sleepCtx(ctx context.Context, d time.Duration) error {
	// Siapkan timer dan pastikan dihentikan saat fungsi selesai.
	t := time.NewTimer(d)
	defer t.Stop()
	// Tunggu timer habis atau konteks batal, mana yang lebih dulu.
	select {
	// Konteks batal: kembalikan galat konteks.
	case <-ctx.Done():
		return ctx.Err()
	// Timer habis: selesai menunggu.
	case <-t.C:
		return nil
	}
}

// Send mengirim satu email, mengulang kegagalan sementara dengan backoff. Galat permanen tidak diulang.
func (m *Mailer) Send(ctx context.Context, msg Message) error {
	// Susun pesan MIME dan validasi penerima. Galat di sini adalah galat permanen (input salah).
	raw, to, err := m.build(msg)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	// Galat percobaan terakhir, dan jeda yang dipakai untuk percobaan berikutnya.
	var last error
	delay := m.backoff
	// Coba kirim sampai jumlah percobaan habis.
	for attempt := 1; attempt <= m.attempts; attempt++ {
		// Satu percobaan kirim.
		last = m.sendOnce(ctx, to, raw)
		// Berhasil: selesai.
		if last == nil {
			return nil
		}
		// Berhenti mencoba bila galat permanen, konteks batal, atau ini percobaan terakhir.
		if errors.Is(last, ErrPermanent) || ctx.Err() != nil || attempt == m.attempts {
			break
		}
		// Tunggu sebelum mencoba lagi. Konteks batal saat menunggu membatalkan seluruh pengiriman.
		if err := m.sleep(ctx, delay); err != nil {
			return err
		}
		// Gandakan jeda untuk percobaan berikutnya.
		delay *= 2
	}
	return last
}

// sendOnce menjalankan satu percakapan SMTP lengkap: dial, TLS, autentikasi, MAIL, RCPT, DATA, lalu QUIT.
func (m *Mailer) sendOnce(ctx context.Context, to string, raw []byte) error {
	// Dialer dengan batas waktu koneksi.
	d := net.Dialer{Timeout: m.dialTimeout}
	var conn net.Conn
	var err error
	// Mode tls: buka koneksi TLS langsung (port 465).
	if m.cfg.TLS == "tls" {
		conn, err = tls.DialWithDialer(&d, "tcp", m.cfg.Addr(), &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		// Selain itu buka TCP biasa; STARTTLS dinegosiasi nanti.
		conn, err = d.DialContext(ctx, "tcp", m.cfg.Addr())
	}
	// Gagal membuka koneksi: tandai sebagai sementara agar bisa dicoba ulang.
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	// Tutup koneksi saat fungsi selesai.
	defer conn.Close()
	// Satu tenggat untuk seluruh percakapan SMTP, dan ikut batal bila ctx batal.
	deadline := m.now().Add(m.attemptTime)
	// Konteks punya tenggat yang lebih dekat: pakai tenggat konteks.
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	// Pasang tenggat pada koneksi.
	_ = conn.SetDeadline(deadline)
	// Bila konteks batal, tutup koneksi agar percakapan yang macet ikut berhenti.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	// Buat klien SMTP di atas koneksi. Galat di sini diklasifikasi (permanen atau sementara).
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return classify(err)
	}
	// Tutup klien SMTP saat fungsi selesai.
	defer c.Close()
	// Mode starttls: minta upgrade ke TLS sebelum mengirim apa pun.
	if m.cfg.TLS == "starttls" {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return classify(err)
		}
	}
	// Ada nama pengguna: lakukan autentikasi.
	if m.cfg.User != "" {
		// PlainAuth menolak mengirim kata sandi lewat koneksi tanpa TLS ke host selain localhost.
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return classify(err)
		}
	}
	// Ambil alamat pengirim murni dari konfigurasi (tanpa nama tampilan).
	from, _ := mail.ParseAddress(m.cfg.From)
	// Kirim perintah MAIL FROM.
	if err := c.Mail(from.Address); err != nil {
		return classify(err)
	}
	// Kirim perintah RCPT TO untuk penerima.
	if err := c.Rcpt(to); err != nil {
		return classify(err)
	}
	// Buka sesi DATA untuk menulis isi pesan.
	w, err := c.Data()
	if err != nil {
		return classify(err)
	}
	// Tulis seluruh pesan MIME.
	if _, err := w.Write(raw); err != nil {
		return classify(err)
	}
	// Tutup sesi DATA; server memberi hasil akhir di sini.
	if err := w.Close(); err != nil {
		return classify(err)
	}
	// Akhiri percakapan dengan QUIT. Galatnya ikut diklasifikasi.
	return classify(c.Quit())
}

// classify menandai galat SMTP 5xx (dan kegagalan autentikasi) sebagai permanen; selebihnya dianggap sementara.
func classify(err error) error {
	// Tidak ada galat: kembalikan nil.
	if err == nil {
		return nil
	}
	// Kode SMTP 5xx dari server berarti penolakan permanen (alamat salah, autentikasi gagal, dll.).
	var tp *textproto.Error
	if errors.As(err, &tp) && tp.Code >= 500 {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	// Server menolak kirim karena koneksi tidak terenkripsi: permanen, karena diulang pun tetap ditolak.
	if strings.Contains(err.Error(), "unencrypted connection") {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	// Galat lain (jaringan, timeout): anggap sementara.
	return err
}

// build menyusun pesan MIME dan memvalidasi penerima. Header dari input luar (To, Subject) tidak boleh memuat
// CR atau LF supaya tidak bisa menyisipkan header lain.
func (m *Mailer) build(msg Message) (raw []byte, to string, err error) {
	// To atau Subject memuat CR atau LF: tolak supaya tidak bisa menyisipkan header lain.
	if strings.ContainsAny(msg.To+msg.Subject, "\r\n") {
		return nil, "", errors.New("header contains line break")
	}
	// Urai alamat penerima. Alamat tidak sah ditolak.
	addr, err := mail.ParseAddress(msg.To)
	if err != nil {
		return nil, "", fmt.Errorf("invalid recipient: %w", err)
	}
	// Teks kosong (hanya spasi): tidak ada isi untuk dikirim.
	if strings.TrimSpace(msg.Text) == "" {
		return nil, "", errors.New("empty text body")
	}
	// Buat ID acak 12 byte untuk Message-ID.
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, "", err
	}
	// Domain Message-ID diambil dari alamat pengirim. Bila tidak bisa diurai, pakai localhost.
	domain := "localhost"
	if from, err := mail.ParseAddress(m.cfg.From); err == nil {
		// Ambil bagian setelah tanda @ sebagai domain.
		if at := strings.LastIndex(from.Address, "@"); at >= 0 {
			domain = from.Address[at+1:]
		}
	}

	// Buffer tempat header dan isi ditulis.
	var buf bytes.Buffer
	// Pembantu: tulis satu header dengan format "Nama: nilai".
	h := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	// Tulis header standar: pengirim, penerima, subjek (di-encode), tanggal, Message-ID, dan versi MIME.
	h("From", m.cfg.From)
	h("To", addr.String())
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", m.now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(id), domain))
	h("MIME-Version", "1.0")

	// Tanpa HTML: kirim sebagai teks biasa saja.
	if msg.HTML == "" {
		h("Content-Type", `text/plain; charset="utf-8"`)
		h("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQP(&buf, msg.Text)
		return buf.Bytes(), addr.Address, nil
	}
	// Ada HTML: buat multipart/alternative dengan dua bagian, teks dan HTML.
	mw := multipart.NewWriter(&buf)
	h("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, mw.Boundary()))
	buf.WriteString("\r\n")
	// Tulis setiap bagian dengan tipe dan isinya.
	for _, part := range []struct{ ctype, body string }{
		{`text/plain; charset="utf-8"`, msg.Text},
		{`text/html; charset="utf-8"`, msg.HTML},
	} {
		// Buat bagian multipart dengan header tipe dan encoding quoted-printable.
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, "", err
		}
		// Tulis isi bagian dalam quoted-printable.
		writeQP(pw, part.body)
	}
	// Tutup batas multipart.
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), addr.Address, nil
}

// writeQP menulis body ke w dalam encoding quoted-printable. Galat tulis diabaikan karena buffer tidak gagal.
func writeQP(w interface{ Write([]byte) (int, error) }, body string) {
	// Bungkus w dengan penulis quoted-printable.
	qp := quotedprintable.NewWriter(w)
	// Tulis body, lalu tutup untuk menuliskan sisa buffer.
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
}
