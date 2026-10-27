package qr_test

import (
	"bytes"
	"image/png"
	"testing"

	"qms/qr-service/internal/qr"
)

// Batas ukuran adalah kontrak publik renderer: default 512 px, di-clamp ke
// rentang 128..2048 px. Nilai ini sengaja ditulis ulang di sini supaya tes
// bersifat black-box dan gagal bila perilaku publik berubah.
const (
	defaultSize = 512
	minSize     = 128
	maxSize     = 2048
)

func TestRender_DefaultProducesValidPNG(t *testing.T) {
	out, err := qr.Render(qr.Options{Data: "https://example.com/join?t=abc"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Width != defaultSize || cfg.Height != defaultSize {
		t.Fatalf("expected %dx%d, got %dx%d", defaultSize, defaultSize, cfg.Width, cfg.Height)
	}
}

func TestRender_SizeIsClamped(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, defaultSize},
		{1, minSize},
		{minSize, minSize},
		{300, 300},
		{maxSize + 500, maxSize},
	}

	for _, tc := range cases {
		out, err := qr.Render(qr.Options{Data: "x", Size: tc.in})
		if err != nil {
			t.Fatalf("Render(size=%d): %v", tc.in, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("decode config: %v", err)
		}
		if cfg.Width != tc.want {
			t.Fatalf("size %d: expected %d, got %d", tc.in, tc.want, cfg.Width)
		}
	}
}

func TestRender_RecoveryLevels(t *testing.T) {
	for _, level := range []string{"", "L", "M", "Q", "H"} {
		if _, err := qr.Render(qr.Options{Data: "x", RecoveryLevel: level}); err != nil {
			t.Fatalf("level %q: %v", level, err)
		}
	}

	if _, err := qr.Render(qr.Options{Data: "x", RecoveryLevel: "Z"}); err == nil {
		t.Fatal("expected error for invalid recovery level")
	}
}

func TestRender_Colors(t *testing.T) {
	if _, err := qr.Render(qr.Options{
		Data:            "x",
		ForegroundColor: "#111827",
		BackgroundColor: "#ffffff",
	}); err != nil {
		t.Fatalf("valid colors: %v", err)
	}

	if _, err := qr.Render(qr.Options{Data: "x", ForegroundColor: "#zzzzzz"}); err == nil {
		t.Fatal("expected error for invalid foreground color")
	}
	if _, err := qr.Render(qr.Options{Data: "x", BackgroundColor: "gggggg"}); err == nil {
		t.Fatal("expected error for invalid background color")
	}
}

func TestRender_EmptyDataIsRejected(t *testing.T) {
	// Handler juga menolak data kosong; di sini dipastikan renderer tidak
	// menghasilkan PNG kosong yang tidak bisa dipindai.
	if _, err := qr.Render(qr.Options{Data: ""}); err == nil {
		t.Fatal("expected error for empty data")
	}
}
