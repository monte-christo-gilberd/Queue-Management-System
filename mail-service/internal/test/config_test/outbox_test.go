package config_test

import (
	"strings"
	"testing"

	"qms/mail-service/internal/config"
)

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MAIL_SERVICE_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SMTP_HOST", "mailpit")
	t.Setenv("SMTP_PORT", "1025")
	t.Setenv("SMTP_FROM", "a@b.test")
	t.Setenv("SMTP_TLS", "none")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("QMS_PUBLIC_ORIGIN", "")
	t.Setenv("OUTBOX_RATE", "")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "")
}

func TestOutbox_DisabledWithoutDatabaseURL(t *testing.T) {
	baseEnv(t)
	cfg, err := config.Load()
	if err != nil || cfg.Outbox.Enabled() {
		t.Fatalf("err=%v enabled=%v", err, cfg.Outbox.Enabled())
	}
}

func TestOutbox_EnabledNeedsAbsolutePublicOrigin(t *testing.T) {
	for _, origin := range []string{"", "localhost:8080", "ftp://h", "/relative"} {
		baseEnv(t)
		t.Setenv("DATABASE_URL", "postgresql://mail_rw:x@db/qms")
		t.Setenv("QMS_PUBLIC_ORIGIN", origin)
		if _, err := config.Load(); err == nil {
			t.Errorf("origin %q seharusnya ditolak", origin)
		}
	}
	baseEnv(t)
	t.Setenv("DATABASE_URL", "postgresql://mail_rw:x@db/qms")
	t.Setenv("QMS_PUBLIC_ORIGIN", "https://qms.example/")
	cfg, err := config.Load()
	if err != nil || !cfg.Outbox.Enabled() || cfg.Outbox.PublicOrigin != "https://qms.example" {
		t.Fatalf("cfg=%+v err=%v", cfg.Outbox, err)
	}
	if cfg.Outbox.Rate != 5 || cfg.Outbox.MaxAttempts != 8 {
		t.Errorf("default salah: %+v", cfg.Outbox)
	}
}

func TestOutbox_RateAndAttemptsValidated(t *testing.T) {
	for _, c := range [][2]string{{"OUTBOX_RATE", "0"}, {"OUTBOX_RATE", "abc"}, {"OUTBOX_RATE", "1000"},
		{"OUTBOX_MAX_ATTEMPTS", "0"}, {"OUTBOX_MAX_ATTEMPTS", "x"}, {"OUTBOX_MAX_ATTEMPTS", "99"}} {
		baseEnv(t)
		t.Setenv(c[0], c[1])
		if _, err := config.Load(); err == nil {
			t.Errorf("%s=%s seharusnya ditolak", c[0], c[1])
		}
	}
}
