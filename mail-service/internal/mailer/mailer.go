// Package mailer mengirim email lewat SMTP: pesan multipart (teks dan HTML), batas waktu, mode TLS, dan
// coba ulang dengan backoff untuk kegagalan sementara. Dipakai notify-service sebagai satu-satunya pengirim email.
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
	To      string
	Subject string
	Text    string
	HTML    string
}

// ErrPermanent membungkus galat yang tidak ada gunanya diulang (alamat ditolak, autentikasi gagal, 5xx).
var ErrPermanent = errors.New("permanent smtp failure")

type Mailer struct {
	cfg config.SMTP
	// Dapat diganti tes.
	dialTimeout time.Duration
	attemptTime time.Duration
	attempts    int
	backoff     time.Duration
	sleep       func(context.Context, time.Duration) error
	now         func() time.Time
}

func New(cfg config.SMTP) *Mailer {
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
	m.attempts, m.backoff = attempts, backoff
	return m
}

// SetSleep mengganti fungsi tunggu antar percobaan. Untuk tes.
func (m *Mailer) SetSleep(f func(context.Context, time.Duration) error) { m.sleep = f }

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Send mengirim satu email, mengulang kegagalan sementara dengan backoff. Galat permanen tidak diulang.
func (m *Mailer) Send(ctx context.Context, msg Message) error {
	raw, to, err := m.build(msg)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	var last error
	delay := m.backoff
	for attempt := 1; attempt <= m.attempts; attempt++ {
		last = m.sendOnce(ctx, to, raw)
		if last == nil {
			return nil
		}
		if errors.Is(last, ErrPermanent) || ctx.Err() != nil || attempt == m.attempts {
			break
		}
		if err := m.sleep(ctx, delay); err != nil {
			return err
		}
		delay *= 2
	}
	return last
}

func (m *Mailer) sendOnce(ctx context.Context, to string, raw []byte) error {
	d := net.Dialer{Timeout: m.dialTimeout}
	var conn net.Conn
	var err error
	if m.cfg.TLS == "tls" {
		conn, err = tls.DialWithDialer(&d, "tcp", m.cfg.Addr(), &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", m.cfg.Addr())
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()
	// Satu tenggat untuk seluruh percakapan SMTP, dan ikut batal bila ctx batal.
	deadline := m.now().Add(m.attemptTime)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return classify(err)
	}
	defer c.Close()
	if m.cfg.TLS == "starttls" {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return classify(err)
		}
	}
	if m.cfg.User != "" {
		// PlainAuth menolak mengirim kata sandi lewat koneksi tanpa TLS ke host selain localhost.
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return classify(err)
		}
	}
	from, _ := mail.ParseAddress(m.cfg.From)
	if err := c.Mail(from.Address); err != nil {
		return classify(err)
	}
	if err := c.Rcpt(to); err != nil {
		return classify(err)
	}
	w, err := c.Data()
	if err != nil {
		return classify(err)
	}
	if _, err := w.Write(raw); err != nil {
		return classify(err)
	}
	if err := w.Close(); err != nil {
		return classify(err)
	}
	return classify(c.Quit())
}

// classify menandai galat SMTP 5xx (dan kegagalan autentikasi) sebagai permanen; selebihnya dianggap sementara.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var tp *textproto.Error
	if errors.As(err, &tp) && tp.Code >= 500 {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	if strings.Contains(err.Error(), "unencrypted connection") {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	return err
}

// build menyusun pesan MIME dan memvalidasi penerima. Header dari input luar (To, Subject) tidak boleh memuat
// CR atau LF supaya tidak bisa menyisipkan header lain.
func (m *Mailer) build(msg Message) (raw []byte, to string, err error) {
	if strings.ContainsAny(msg.To+msg.Subject, "\r\n") {
		return nil, "", errors.New("header contains line break")
	}
	addr, err := mail.ParseAddress(msg.To)
	if err != nil {
		return nil, "", fmt.Errorf("invalid recipient: %w", err)
	}
	if strings.TrimSpace(msg.Text) == "" {
		return nil, "", errors.New("empty text body")
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, "", err
	}
	domain := "localhost"
	if from, err := mail.ParseAddress(m.cfg.From); err == nil {
		if at := strings.LastIndex(from.Address, "@"); at >= 0 {
			domain = from.Address[at+1:]
		}
	}

	var buf bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	h("From", m.cfg.From)
	h("To", addr.String())
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", m.now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(id), domain))
	h("MIME-Version", "1.0")

	if msg.HTML == "" {
		h("Content-Type", `text/plain; charset="utf-8"`)
		h("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQP(&buf, msg.Text)
		return buf.Bytes(), addr.Address, nil
	}
	mw := multipart.NewWriter(&buf)
	h("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, mw.Boundary()))
	buf.WriteString("\r\n")
	for _, part := range []struct{ ctype, body string }{
		{`text/plain; charset="utf-8"`, msg.Text},
		{`text/html; charset="utf-8"`, msg.HTML},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, "", err
		}
		writeQP(pw, part.body)
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), addr.Address, nil
}

func writeQP(w interface{ Write([]byte) (int, error) }, body string) {
	qp := quotedprintable.NewWriter(w)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
}
