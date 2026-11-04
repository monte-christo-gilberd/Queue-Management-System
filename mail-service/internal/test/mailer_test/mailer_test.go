package mailer_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"qms/mail-service/internal/config"
	"qms/mail-service/internal/mailer"
)

// fakeSMTP: server SMTP minimal. script[i] menentukan balasan untuk sesi ke-i terhadap perintah yang
// ditolak: "" = terima semua, "rcpt4xx", "rcpt5xx", "greet4xx".
type fakeSMTP struct {
	ln       net.Listener
	mu       sync.Mutex
	sessions int
	script   []string
	got      []received
	hangOnce bool
}

type received struct {
	From, To string
	Data     string
}

func startSMTP(t *testing.T, script ...string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, script: script}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) cfg() config.SMTP {
	addr := f.ln.Addr().(*net.TCPAddr)
	return config.SMTP{Host: "127.0.0.1", Port: addr.Port, From: "QMS <no-reply@qms.local>", TLS: "none"}
}

func (f *fakeSMTP) received() []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]received(nil), f.got...)
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	f.mu.Lock()
	mode := ""
	if f.sessions < len(f.script) {
		mode = f.script[f.sessions]
	}
	f.sessions++
	f.mu.Unlock()
	r := bufio.NewReader(c)
	w := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	if mode == "greet4xx" {
		w("421 try again later")
		return
	}
	if mode == "hang" {
		time.Sleep(3 * time.Second)
		return
	}
	w("220 fake ESMTP")
	var cur received
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250 fake")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			cur.From = strings.Trim(strings.TrimSpace(line[len("MAIL FROM:"):]), "<>")
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if mode == "rcpt4xx" {
				w("451 temporary failure")
				continue
			}
			if mode == "rcpt5xx" {
				w("550 no such user")
				continue
			}
			cur.To = strings.Trim(strings.TrimSpace(line[len("RCPT TO:"):]), "<>")
			w("250 ok")
		case cmd == "DATA":
			w("354 go ahead")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(strings.TrimPrefix(l, "."))
			}
			cur.Data = sb.String()
			f.mu.Lock()
			f.got = append(f.got, cur)
			f.mu.Unlock()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func msg() mailer.Message {
	return mailer.Message{To: "budi@example.com", Subject: "Giliran Anda: A-003", Text: "Nomor A-003 dipanggil.", HTML: "<p>Nomor <b>A-003</b> dipanggil.</p>"}
}

func fast(m *mailer.Mailer) *mailer.Mailer { return m.WithRetry(3, time.Millisecond) }

func TestSend_DeliversMultipartMessage(t *testing.T) {
	f := startSMTP(t)
	if err := fast(mailer.New(f.cfg())).Send(context.Background(), msg()); err != nil {
		t.Fatal(err)
	}
	got := f.received()
	if len(got) != 1 {
		t.Fatalf("want 1 message, got %d", len(got))
	}
	if got[0].From != "no-reply@qms.local" || got[0].To != "budi@example.com" {
		t.Fatalf("envelope %+v", got[0])
	}
	m, err := mail.ReadMessage(strings.NewReader(got[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.Get("From") != "QMS <no-reply@qms.local>" || m.Header.Get("MIME-Version") != "1.0" || m.Header.Get("Message-ID") == "" || m.Header.Get("Date") == "" {
		t.Fatalf("headers %v", m.Header)
	}
	subj, _ := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if subj != "Giliran Anda: A-003" {
		t.Fatalf("subject %q", subj)
	}
	mt, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content-type %q %v", mt, err)
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	var kinds []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(quotedprintable.NewReader(p))
		kinds = append(kinds, strings.Split(p.Header.Get("Content-Type"), ";")[0]+"|"+string(body))
	}
	if len(kinds) != 2 || !strings.HasPrefix(kinds[0], "text/plain|Nomor A-003") || !strings.HasPrefix(kinds[1], "text/html|<p>Nomor <b>A-003</b>") {
		t.Fatalf("parts %q", kinds)
	}
}

func TestSend_UTF8AndPlainTextOnly(t *testing.T) {
	f := startSMTP(t)
	err := fast(mailer.New(f.cfg())).Send(context.Background(), mailer.Message{To: "a@b.co", Subject: "Antrian Kopi Kenangan ☕ — nomor 7", Text: "Sampai jumpa di kafé 😊"})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := mail.ReadMessage(strings.NewReader(f.received()[0].Data))
	subj, _ := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if subj != "Antrian Kopi Kenangan ☕ — nomor 7" {
		t.Fatalf("subject %q", subj)
	}
	if !strings.HasPrefix(m.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("content-type %q", m.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(quotedprintable.NewReader(m.Body))
	if !strings.Contains(string(body), "kafé 😊") {
		t.Fatalf("body %q", body)
	}
}

func TestSend_RejectsHeaderInjectionAndBadInput(t *testing.T) {
	f := startSMTP(t)
	m := fast(mailer.New(f.cfg()))
	for name, in := range map[string]mailer.Message{
		"subject CRLF": {To: "a@b.co", Subject: "x\r\nBcc: evil@x.io", Text: "t"},
		"to CRLF":      {To: "a@b.co\r\nBcc: evil@x.io", Subject: "s", Text: "t"},
		"to invalid":   {To: "bukan-email", Subject: "s", Text: "t"},
		"text kosong":  {To: "a@b.co", Subject: "s", Text: "  "},
	} {
		err := m.Send(context.Background(), in)
		if !errors.Is(err, mailer.ErrPermanent) {
			t.Errorf("%s: want permanent error, got %v", name, err)
		}
	}
	if n := len(f.received()); n != 0 {
		t.Fatalf("nothing should reach the server, got %d", n)
	}
}

func TestSend_RetriesTransientThenSucceeds(t *testing.T) {
	f := startSMTP(t, "greet4xx", "rcpt4xx", "")
	if err := fast(mailer.New(f.cfg())).Send(context.Background(), msg()); err != nil {
		t.Fatal(err)
	}
	if len(f.received()) != 1 || f.sessions != 3 {
		t.Fatalf("messages=%d sessions=%d", len(f.received()), f.sessions)
	}
}

func TestSend_GivesUpAfterAttempts(t *testing.T) {
	f := startSMTP(t, "rcpt4xx", "rcpt4xx", "rcpt4xx", "")
	err := fast(mailer.New(f.cfg())).Send(context.Background(), msg())
	if err == nil || errors.Is(err, mailer.ErrPermanent) {
		t.Fatalf("want transient error, got %v", err)
	}
	if f.sessions != 3 || len(f.received()) != 0 {
		t.Fatalf("sessions=%d messages=%d", f.sessions, len(f.received()))
	}
}

func TestSend_DoesNotRetryPermanentRejection(t *testing.T) {
	f := startSMTP(t, "rcpt5xx", "")
	err := fast(mailer.New(f.cfg())).Send(context.Background(), msg())
	if !errors.Is(err, mailer.ErrPermanent) {
		t.Fatalf("want permanent, got %v", err)
	}
	if f.sessions != 1 {
		t.Fatalf("should not retry, sessions=%d", f.sessions)
	}
}

func TestSend_ConnectionRefusedIsTransientAndRetried(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := config.SMTP{Host: "127.0.0.1", Port: port, From: "no-reply@qms.local", TLS: "none"}
	var sleeps int
	m := mailer.New(cfg).WithRetry(3, time.Millisecond)
	m.SetSleep(func(context.Context, time.Duration) error { sleeps++; return nil })
	err := m.Send(context.Background(), msg())
	if err == nil || errors.Is(err, mailer.ErrPermanent) {
		t.Fatalf("got %v", err)
	}
	if sleeps != 2 {
		t.Fatalf("want 2 backoff sleeps between 3 attempts, got %d", sleeps)
	}
}

func TestSend_BackoffDoubles(t *testing.T) {
	f := startSMTP(t, "rcpt4xx", "rcpt4xx", "rcpt4xx")
	var delays []time.Duration
	m := mailer.New(f.cfg()).WithRetry(3, 100*time.Millisecond)
	m.SetSleep(func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil })
	_ = m.Send(context.Background(), msg())
	if len(delays) != 2 || delays[0] != 100*time.Millisecond || delays[1] != 200*time.Millisecond {
		t.Fatalf("delays %v", delays)
	}
}

func TestSend_ContextCancelStopsAHangingServer(t *testing.T) {
	f := startSMTP(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := fast(mailer.New(f.cfg())).Send(ctx, msg())
	if err == nil {
		t.Fatal("want error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v, should stop near the context deadline", time.Since(start))
	}
}

func TestRender_TestTemplateEscapesHTMLAndFillsText(t *testing.T) {
	r, err := mailer.Render("test", map[string]any{
		"To":     `<script>alert(1)</script>@x.io`,
		"SentAt": "4 Nov 2026",
		"Link":   "http://localhost:8080/reset-password?token=abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "Email uji dari Antrian" {
		t.Fatalf("subject %q", r.Subject)
	}
	if strings.Contains(r.HTML, "<script>") || !strings.Contains(r.HTML, "&lt;script&gt;") {
		t.Fatalf("HTML must escape user data: %s", r.HTML)
	}
	if !strings.Contains(r.HTML, `href="http://localhost:8080/reset-password?token=abc"`) || !strings.Contains(r.HTML, "<!doctype html>") {
		t.Fatalf("HTML missing link or layout: %s", r.HTML)
	}
	if !strings.Contains(r.Text, "token=abc") || strings.Contains(r.Text, "<html") {
		t.Fatalf("text %q", r.Text)
	}
}

func TestRender_UnknownTemplateIsAnError(t *testing.T) {
	if _, err := mailer.Render("tidak-ada", map[string]any{}); !errors.Is(err, mailer.ErrUnknownTemplate) {
		t.Fatal("want ErrUnknownTemplate")
	}
}

func TestRender_MissingDataIsAnErrorNotNoValue(t *testing.T) {
	_, err := mailer.Render("test", map[string]any{"To": "a@b.co"})
	if !errors.Is(err, mailer.ErrBadData) {
		t.Fatalf("want ErrBadData, got %v", err)
	}
}

func TestHas_OnlyCompleteTemplatesWithSafeNames(t *testing.T) {
	if !mailer.Has("test") {
		t.Fatal("test must exist")
	}
	for _, name := range []string{"", "layout", "../layout", "test/../test", "TEST", "tidak-ada", "test.txt", "a b"} {
		if mailer.Has(name) {
			t.Errorf("%q must not be a valid template", name)
		}
	}
}

func TestRender_PasswordResetHasLinkAndEscapesName(t *testing.T) {
	link := "https://app.example/reset-password?token=abc_DEF-123"
	r, err := mailer.Render("password-reset", map[string]any{"Name": `<img src=x onerror=1>`, "Link": link, "Minutes": 30})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, link) || !strings.Contains(r.Text, "30 menit") {
		t.Errorf("teks tanpa tautan/masa berlaku: %q", r.Text)
	}
	if !strings.Contains(r.HTML, `href="`+link+`"`) {
		t.Errorf("HTML tanpa tautan utuh: %q", r.HTML)
	}
	if strings.Contains(r.HTML, "<img src=x") {
		t.Errorf("nama pengguna tidak di-escape di HTML")
	}
	if strings.ContainsAny(r.Subject, "\r\n") || r.Subject == "" {
		t.Errorf("subjek tidak sah: %q", r.Subject)
	}
	if _, err := mailer.Render("password-reset", map[string]any{"Name": "x"}); !errors.Is(err, mailer.ErrBadData) {
		t.Errorf("data kurang harus ErrBadData, dapat %v", err)
	}
}
