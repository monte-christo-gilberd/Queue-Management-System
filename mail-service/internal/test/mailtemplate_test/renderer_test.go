package mailtemplate_test

import (
	"errors"
	"strings"
	"testing"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/mailtemplate"
)

func newRenderer() *mailtemplate.Renderer { return mailtemplate.New() }

func TestRender_TestTemplateEscapesHTMLAndFillsText(t *testing.T) {
	r, err := newRenderer().Render("test", map[string]any{
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
	if _, err := newRenderer().Render("tidak-ada", map[string]any{}); !errors.Is(err, apperror.ErrUnknownTemplate) {
		t.Fatal("want ErrUnknownTemplate")
	}
}

func TestRender_MissingDataIsAnErrorNotNoValue(t *testing.T) {
	_, err := newRenderer().Render("test", map[string]any{"To": "a@b.co"})
	if !errors.Is(err, apperror.ErrBadData) {
		t.Fatalf("want ErrBadData, got %v", err)
	}
}

func TestHas_OnlyCompleteTemplatesWithSafeNames(t *testing.T) {
	if !newRenderer().Has("test") {
		t.Fatal("test must exist")
	}
	for _, name := range []string{"", "layout", "../layout", "test/../test", "TEST", "tidak-ada", "test.txt", "a b"} {
		if newRenderer().Has(name) {
			t.Errorf("%q must not be a valid template", name)
		}
	}
}

func TestRender_PasswordResetHasLinkAndEscapesName(t *testing.T) {
	link := "https://app.example/reset-password?token=abc_DEF-123"
	r, err := newRenderer().Render("password-reset", map[string]any{"Name": `<img src=x onerror=1>`, "Link": link, "Minutes": 30})
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
	if _, err := newRenderer().Render("password-reset", map[string]any{"Name": "x"}); !errors.Is(err, apperror.ErrBadData) {
		t.Errorf("data kurang harus ErrBadData, dapat %v", err)
	}
}

func TestRender_EmailVerifyHasLinkAndEscapesName(t *testing.T) {
	link := "https://app.example/verify-email?token=abc_DEF-123"
	r, err := newRenderer().Render("email-verify", map[string]any{"Name": `<img src=x onerror=1>`, "Link": link, "Hours": 24})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, link) || !strings.Contains(r.Text, "24 jam") {
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
	if _, err := newRenderer().Render("email-verify", map[string]any{"Name": "x"}); !errors.Is(err, apperror.ErrBadData) {
		t.Errorf("data kurang harus ErrBadData, dapat %v", err)
	}
}

func TestRender_QueueClosedHasQueueAndLink(t *testing.T) {
	if !newRenderer().Has("queue-closed") {
		t.Fatal("queue-closed harus ada")
	}
	link := "https://app.example/riwayat"
	r, err := newRenderer().Render("queue-closed", map[string]any{"Name": `<img src=x onerror=1>`, "Queue": "Loket A", "Link": link})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, link) || !strings.Contains(r.Text, "Loket A") {
		t.Errorf("teks tanpa tautan/antrian: %q", r.Text)
	}
	if !strings.Contains(r.HTML, `href="`+link+`"`) || strings.Contains(r.HTML, "<img src=x") {
		t.Errorf("HTML tanpa tautan utuh atau nama tidak di-escape: %q", r.HTML)
	}
	if strings.ContainsAny(r.Subject, "\r\n") || r.Subject != "Antrian Loket A sudah ditutup" {
		t.Errorf("subjek tidak sah: %q", r.Subject)
	}
	if _, err := newRenderer().Render("queue-closed", map[string]any{"Name": "x"}); !errors.Is(err, apperror.ErrBadData) {
		t.Errorf("data kurang harus ErrBadData, dapat %v", err)
	}
}

func TestRender_EmailVerifiedHasLinkAndEscapesName(t *testing.T) {
	if !newRenderer().Has("email-verified") {
		t.Fatal("email-verified harus ada")
	}
	link := "https://app.example/"
	r, err := newRenderer().Render("email-verified", map[string]any{"Name": `<img src=x onerror=1>`, "Link": link})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, link) || !strings.Contains(r.HTML, `href="`+link+`"`) {
		t.Errorf("tanpa tautan utuh: %q", r.HTML)
	}
	if strings.Contains(r.HTML, "<img src=x") {
		t.Errorf("nama pengguna tidak di-escape di HTML")
	}
	if strings.ContainsAny(r.Subject, "\r\n") || r.Subject == "" {
		t.Errorf("subjek tidak sah: %q", r.Subject)
	}
	if _, err := newRenderer().Render("email-verified", map[string]any{"Name": "x"}); !errors.Is(err, apperror.ErrBadData) {
		t.Errorf("data kurang harus ErrBadData, dapat %v", err)
	}
}
