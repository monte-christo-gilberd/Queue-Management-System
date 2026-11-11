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

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/config"
	"qms/mail-service/internal/model"
)

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
	// Kembalikan Mailer yang sama.
	return m
}

// SetSleep mengganti fungsi tunggu antar percobaan. Untuk tes.
func (m *Mailer) SetSleep(f func(context.Context, time.Duration) error) { m.sleep = f }

// sleepCtx menunggu d, atau berhenti lebih cepat bila ctx batal (dan mengembalikan galat konteks).
func sleepCtx(ctx context.Context, d time.Duration) error {
	// Siapkan timer dan pastikan dihentikan saat fungsi selesai.
	t := time.NewTimer(d)
	// Hentikan timer bila fungsi selesai lebih awal agar tidak bocor.
	defer t.Stop()
	// Tunggu timer habis atau konteks batal, mana yang lebih dulu.
	select {
	// Konteks batal: kembalikan galat konteks.
	case <-ctx.Done():
		// Konteks batal: berhenti menunggu dan beri tahu penyebabnya.
		return ctx.Err()
	// Timer habis: selesai menunggu.
	case <-t.C:
		// Selesai menunggu penuh.
		return nil
	}
}

// Send mengirim satu email, mengulang kegagalan sementara dengan backoff. Galat permanen tidak diulang.
func (m *Mailer) Send(ctx context.Context, msg model.Message) error {
	// Susun pesan MIME dan validasi penerima. Galat di sini adalah galat permanen (input salah).
	raw, to, err := m.build(msg)
	// Susun pesan lebih dulu agar input salah ketahuan sebelum membuka koneksi.
	if err != nil {
		// Input salah tidak akan berhasil bila diulang, jadi tandai permanen.
		return fmt.Errorf("%w: %v", apperror.ErrPermanent, err)
	}
	// Galat percobaan terakhir, dan jeda yang dipakai untuk percobaan berikutnya.
	var last error
	// Jeda awal untuk percobaan berikutnya.
	delay := m.backoff
	// Coba kirim sampai jumlah percobaan habis.
	for attempt := 1; attempt <= m.attempts; attempt++ {
		// Satu percobaan kirim.
		last = m.sendOnce(ctx, to, raw)
		// Berhasil: selesai.
		if last == nil {
			// Email terkirim: tidak perlu mencoba lagi.
			return nil
		}
		// Berhenti mencoba bila galat permanen, konteks batal, atau ini percobaan terakhir.
		if errors.Is(last, apperror.ErrPermanent) || ctx.Err() != nil || attempt == m.attempts {
			// Keluar dari loop dan kembalikan galat terakhir. Mengulang galat permanen hanya membuang waktu.
			break
		}
		// Tunggu sebelum mencoba lagi. Konteks batal saat menunggu membatalkan seluruh pengiriman.
		if err := m.sleep(ctx, delay); err != nil {
			// Pengiriman dibatalkan saat menunggu: kembalikan galat konteks.
			return err
		}
		// Gandakan jeda untuk percobaan berikutnya.
		delay *= 2
	}
	// Semua percobaan gagal: kembalikan galat terakhir.
	return last
}

// sendOnce menjalankan satu percakapan SMTP lengkap: dial, TLS, autentikasi, MAIL, RCPT, DATA, lalu QUIT.
func (m *Mailer) sendOnce(ctx context.Context, to string, raw []byte) error {
	// Dialer dengan batas waktu koneksi.
	d := net.Dialer{Timeout: m.dialTimeout}
	// Koneksi dan galat dibuat di luar if agar bisa dipakai kedua cabang.
	var conn net.Conn
	var err error
	// Mode tls: buka koneksi TLS langsung (port 465).
	if m.cfg.TLS == "tls" {
		// Koneksi TLS penuh. TLS 1.2 adalah versi minimum yang aman.
		conn, err = tls.DialWithDialer(&d, "tcp", m.cfg.Addr(), &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		// Selain itu buka TCP biasa; STARTTLS dinegosiasi nanti.
		conn, err = d.DialContext(ctx, "tcp", m.cfg.Addr())
	}
	// Gagal membuka koneksi: tandai sebagai sementara agar bisa dicoba ulang.
	if err != nil {
		// Galat dial tidak diklasifikasi: gangguan jaringan dianggap sementara.
		return fmt.Errorf("smtp dial: %w", err)
	}
	// Tutup koneksi saat fungsi selesai.
	defer conn.Close()
	// Satu tenggat untuk seluruh percakapan SMTP, dan ikut batal bila ctx batal.
	deadline := m.now().Add(m.attemptTime)
	// Konteks punya tenggat yang lebih dekat: pakai tenggat konteks.
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		// Tenggat konteks lebih awal, jadi pakai tenggat itu.
		deadline = dl
	}
	// Pasang tenggat pada koneksi.
	_ = conn.SetDeadline(deadline)
	// Bila konteks batal, tutup koneksi agar percakapan yang macet ikut berhenti.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	// Lepas pendaftaran callback bila fungsi selesai tanpa konteks batal.
	defer stop()

	// Buat klien SMTP di atas koneksi. Galat di sini diklasifikasi (permanen atau sementara).
	c, err := smtp.NewClient(conn, m.cfg.Host)
	// Gagal membuat klien (mis. server menyapa dengan kode 5xx).
	if err != nil {
		// Klasifikasi galat: 5xx permanen, selain itu sementara.
		return classify(err)
	}
	// Tutup klien SMTP saat fungsi selesai.
	defer c.Close()
	// Mode starttls: minta upgrade ke TLS sebelum mengirim apa pun.
	if m.cfg.TLS == "starttls" {
		// Naikkan koneksi ke TLS. Sandi tidak boleh dikirim sebelum koneksi terenkripsi.
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			// Gagal negosiasi TLS: klasifikasi galatnya.
			return classify(err)
		}
	}
	// Ada nama pengguna: lakukan autentikasi.
	if m.cfg.User != "" {
		// PlainAuth menolak mengirim kata sandi lewat koneksi tanpa TLS ke host selain localhost.
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			// Autentikasi ditolak (5xx) berarti kredensial salah, bukan gangguan sementara.
			return classify(err)
		}
	}
	// Ambil alamat pengirim murni dari konfigurasi (tanpa nama tampilan).
	from, _ := mail.ParseAddress(m.cfg.From)
	// Kirim perintah MAIL FROM.
	if err := c.Mail(from.Address); err != nil {
		// Pengirim ditolak server.
		return classify(err)
	}
	// Kirim perintah RCPT TO untuk penerima.
	if err := c.Rcpt(to); err != nil {
		// Penerima ditolak: 5xx berarti alamat tidak ada atau tidak diterima.
		return classify(err)
	}
	// Buka sesi DATA untuk menulis isi pesan.
	w, err := c.Data()
	// Server tidak mau menerima isi pesan.
	if err != nil {
		return classify(err)
	}
	// Tulis seluruh pesan MIME.
	if _, err := w.Write(raw); err != nil {
		// Gagal menulis isi pesan ke koneksi.
		return classify(err)
	}
	// Tutup sesi DATA; server memberi hasil akhir di sini.
	if err := w.Close(); err != nil {
		// Server menolak pesan setelah DATA selesai (mis. dianggap spam).
		return classify(err)
	}
	// Akhiri percakapan dengan QUIT. Galatnya ikut diklasifikasi.
	return classify(c.Quit())
}

// classify menandai galat SMTP 5xx (dan kegagalan autentikasi) sebagai permanen; selebihnya dianggap sementara.
func classify(err error) error {
	// Tidak ada galat: kembalikan nil.
	if err == nil {
		// Bukan galat: tidak ada yang diklasifikasi.
		return nil
	}
	// Kode SMTP 5xx dari server berarti penolakan permanen (alamat salah, autentikasi gagal, dll.).
	var tp *textproto.Error
	// Hanya kode 5xx yang permanen. Kode 4xx adalah penolakan sementara (mis. antrean penuh, greylisting), jadi dibiarkan agar dicoba ulang.
	if errors.As(err, &tp) && tp.Code >= 500 {
		// Bungkus dengan ErrPermanent supaya pemanggil berhenti mengulang.
		return fmt.Errorf("%w: %v", apperror.ErrPermanent, err)
	}
	// Server menolak kirim karena koneksi tidak terenkripsi: permanen, karena diulang pun tetap ditolak.
	if strings.Contains(err.Error(), "unencrypted connection") {
		// Dibungkus permanen agar tidak dicoba ulang.
		return fmt.Errorf("%w: %v", apperror.ErrPermanent, err)
	}
	// Galat lain (jaringan, timeout): anggap sementara.
	return err
}

// build menyusun pesan MIME dan memvalidasi penerima. Header dari input luar (To, Subject) tidak boleh memuat
// CR atau LF supaya tidak bisa menyisipkan header lain.
func (m *Mailer) build(msg model.Message) (raw []byte, to string, err error) {
	// To atau Subject memuat CR atau LF: tolak supaya tidak bisa menyisipkan header lain.
	if strings.ContainsAny(msg.To+msg.Subject, "\r\n") {
		// Ditolak, bukan dibersihkan: pesan yang sengaja memuat baris baru dianggap input berbahaya (header injection).
		return nil, "", errors.New("header contains line break")
	}
	// Urai alamat penerima. Alamat tidak sah ditolak.
	addr, err := mail.ParseAddress(msg.To)
	// Alamat tidak sah tidak akan pernah berhasil dikirim.
	if err != nil {
		return nil, "", fmt.Errorf("invalid recipient: %w", err)
	}
	// Teks kosong (hanya spasi): tidak ada isi untuk dikirim.
	if strings.TrimSpace(msg.Text) == "" {
		// Isi teks selalu wajib karena menjadi bagian cadangan untuk klien tanpa HTML.
		return nil, "", errors.New("empty text body")
	}
	// Buat ID acak 12 byte untuk Message-ID.
	id := make([]byte, 12)
	// Pembuatan angka acak gagal. Kembalikan galat karena Message-ID harus unik.
	if _, err := rand.Read(id); err != nil {
		// Kembalikan galat sumber acak.
		return nil, "", err
	}
	// Domain Message-ID diambil dari alamat pengirim. Bila tidak bisa diurai, pakai localhost.
	domain := "localhost"
	// Pengirim dari konfigurasi dipakai agar domain Message-ID sama dengan alamat From.
	if from, err := mail.ParseAddress(m.cfg.From); err == nil {
		// Ambil bagian setelah tanda @ sebagai domain.
		if at := strings.LastIndex(from.Address, "@"); at >= 0 {
			// Pakai domain dari alamat pengirim.
			domain = from.Address[at+1:]
		}
	}

	// Buffer tempat header dan isi ditulis.
	var buf bytes.Buffer
	// Pembantu: tulis satu header dengan format "Nama: nilai".
	h := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	// Tulis header standar: pengirim, penerima, subjek (di-encode), tanggal, Message-ID, dan versi MIME.
	h("From", m.cfg.From)
	// Penerima ditulis ulang dari hasil urai agar bentuknya aman.
	h("To", addr.String())
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", m.now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(id), domain))
	h("MIME-Version", "1.0")

	// Tanpa HTML: kirim sebagai teks biasa saja.
	if msg.HTML == "" {
		// Teks biasa memakai quoted-printable agar karakter non-ASCII aman lewat SMTP.
		h("Content-Type", `text/plain; charset="utf-8"`)
		h("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQP(&buf, msg.Text)
		// Kembalikan pesan teks biasa dan alamat murni untuk RCPT TO.
		return buf.Bytes(), addr.Address, nil
	}
	// Ada HTML: buat multipart/alternative dengan dua bagian, teks dan HTML.
	mw := multipart.NewWriter(&buf)
	// multipart/alternative: klien memilih bagian terbaik yang bisa ditampilkan.
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
		// Gagal membuat bagian multipart.
		if err != nil {
			// Kembalikan galat pembuatan bagian.
			return nil, "", err
		}
		// Tulis isi bagian dalam quoted-printable.
		writeQP(pw, part.body)
	}
	// Tutup batas multipart.
	if err := mw.Close(); err != nil {
		// Gagal menutup batas multipart.
		return nil, "", err
	}
	// Kembalikan pesan lengkap dan alamat murni untuk RCPT TO.
	return buf.Bytes(), addr.Address, nil
}

// writeQP menulis body ke w dalam encoding quoted-printable. Galat tulis diabaikan karena buffer tidak gagal.
func writeQP(w interface{ Write([]byte) (int, error) }, body string) {
	// Bungkus w dengan penulis quoted-printable.
	qp := quotedprintable.NewWriter(w)
	// Tulis body, lalu tutup untuk menuliskan sisa buffer.
	_, _ = qp.Write([]byte(body))
	// Close menuliskan sisa buffer. Galat diabaikan karena target adalah buffer di memori.
	_ = qp.Close()
}
