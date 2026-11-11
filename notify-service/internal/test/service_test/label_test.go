package service_test

import (
	"testing"

	"qms/notify-service/internal/service"
)

func TestLabel(t *testing.T) {
	cases := []struct {
		prefix string
		n      int
		want   string
	}{
		{"A", 7, "A-007"},
		{"", 7, "A-007"},
		{"VIP", 12, "VIP-012"},
		{"B2", 1234, "B2-1234"},
	}
	for _, c := range cases {
		if got := service.Label(c.prefix, c.n); got != c.want {
			t.Fatalf("Label(%q,%d)=%s, want %s", c.prefix, c.n, got, c.want)
		}
	}
}
