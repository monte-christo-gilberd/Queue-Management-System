package config_test

import (
	"strings"
	"testing"

	"qms/mail-service/internal/config"
)

func setSMTP(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_FROM", "SMTP_USER", "SMTP_PASSWORD", "SMTP_TLS"} {
		t.Setenv(k, kv[k])
	}
}

func valid() map[string]string {
	return map[string]string{"SMTP_HOST": "mailpit", "SMTP_PORT": "1025", "SMTP_FROM": "QMS <no-reply@qms.local>"}
}

func TestLoadSMTP_ValidDefaultsToNoTLS(t *testing.T) {
	setSMTP(t, valid())
	s, err := config.LoadSMTP()
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "mailpit" || s.Port != 1025 || s.TLS != "none" || s.Addr() != "mailpit:1025" {
		t.Fatalf("got %+v", s)
	}
}

func TestLoadSMTP_FailsFast(t *testing.T) {
	cases := map[string]struct {
		mutate func(map[string]string)
		want   string
	}{
		"host kosong":       {func(m map[string]string) { delete(m, "SMTP_HOST") }, "SMTP_HOST"},
		"from kosong":       {func(m map[string]string) { delete(m, "SMTP_FROM") }, "SMTP_FROM"},
		"from bukan alamat": {func(m map[string]string) { m["SMTP_FROM"] = "bukan alamat" }, "SMTP_FROM"},
		"port kosong":       {func(m map[string]string) { delete(m, "SMTP_PORT") }, "SMTP_PORT"},
		"port huruf":        {func(m map[string]string) { m["SMTP_PORT"] = "abc" }, "SMTP_PORT"},
		"port di luar":      {func(m map[string]string) { m["SMTP_PORT"] = "70000" }, "SMTP_PORT"},
		"tls salah":         {func(m map[string]string) { m["SMTP_TLS"] = "ssl" }, "SMTP_TLS"},
		"user tanpa sandi":  {func(m map[string]string) { m["SMTP_USER"] = "u" }, "together"},
		"sandi tanpa user":  {func(m map[string]string) { m["SMTP_PASSWORD"] = "p" }, "together"},
	}
	for name, c := range cases {
		m := valid()
		c.mutate(m)
		setSMTP(t, m)
		_, err := config.LoadSMTP()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err=%v, want mention of %q", name, err, c.want)
		}
	}
}

func TestLoadSMTP_AcceptsTLSModesCaseInsensitive(t *testing.T) {
	for _, mode := range []string{"none", "STARTTLS", "tls"} {
		m := valid()
		m["SMTP_TLS"] = mode
		setSMTP(t, m)
		if _, err := config.LoadSMTP(); err != nil {
			t.Errorf("%s: %v", mode, err)
		}
	}
	m := valid()
	m["SMTP_USER"], m["SMTP_PASSWORD"] = "u", "p"
	setSMTP(t, m)
	if _, err := config.LoadSMTP(); err != nil {
		t.Errorf("auth lengkap: %v", err)
	}
}

func TestLoad_RequiresLongServiceToken(t *testing.T) {
	setSMTP(t, valid())
	for _, tok := range []string{"", "pendek", strings.Repeat("a", config.MinTokenLen-1)} {
		t.Setenv("MAIL_SERVICE_TOKEN", tok)
		if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "MAIL_SERVICE_TOKEN") {
			t.Errorf("token %q: err=%v", tok, err)
		}
	}
	t.Setenv("MAIL_SERVICE_TOKEN", strings.Repeat("a", config.MinTokenLen))
	t.Setenv("PORT", "")
	cfg, err := config.Load()
	if err != nil || cfg.Port != "8092" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
