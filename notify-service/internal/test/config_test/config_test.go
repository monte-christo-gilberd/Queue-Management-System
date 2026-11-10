package config_test

import (
	"strings"
	"testing"

	"qms/notify-service/internal/config"
)

var envNames = []string{"DATABASE_URL", "VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "VAPID_SUBJECT"}

// setAll fills every required variable with a distinct value.
func setAll(t *testing.T) {
	t.Helper()
	for _, n := range envNames {
		t.Setenv(n, "v-"+n)
	}
}

func TestLoadReadsAllVariables(t *testing.T) {
	setAll(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := config.Config{
		DatabaseURL:     "v-DATABASE_URL",
		VAPIDPublicKey:  "v-VAPID_PUBLIC_KEY",
		VAPIDPrivateKey: "v-VAPID_PRIVATE_KEY",
		VAPIDSubject:    "v-VAPID_SUBJECT",
	}
	if cfg != want {
		t.Fatalf("cfg=%+v, want %+v", cfg, want)
	}
}

func TestLoadRejectsEmptyRequiredVariable(t *testing.T) {
	for _, name := range envNames {
		t.Run(name, func(t *testing.T) {
			setAll(t)
			t.Setenv(name, "")
			_, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("err=%v, want error naming %s", err, name)
			}
		})
	}
}
