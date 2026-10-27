package qr

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	defaultSize = 512
	minSize     = 128
	maxSize     = 2048
)

type Options struct {
	Data            string
	Size            int
	RecoveryLevel   string
	ForegroundColor string
	BackgroundColor string
}

// Render draws opts.Data as a PNG QR code, applying sane defaults/bounds for
// size and recovery level and optional #rrggbb foreground/background colors.
func Render(opts Options) ([]byte, error) {
	size := opts.Size
	if size <= 0 {
		size = defaultSize
	}
	if size < minSize {
		size = minSize
	}
	if size > maxSize {
		size = maxSize
	}

	level, err := parseRecoveryLevel(opts.RecoveryLevel)
	if err != nil {
		return nil, err
	}

	code, err := qrcode.New(opts.Data, level)
	if err != nil {
		return nil, fmt.Errorf("could not build qr code: %w", err)
	}

	if opts.ForegroundColor != "" {
		fg, err := parseHexColor(opts.ForegroundColor)
		if err != nil {
			return nil, fmt.Errorf("invalid foreground color: %w", err)
		}
		code.ForegroundColor = fg
	}

	if opts.BackgroundColor != "" {
		bg, err := parseHexColor(opts.BackgroundColor)
		if err != nil {
			return nil, fmt.Errorf("invalid background color: %w", err)
		}
		code.BackgroundColor = bg
	}

	png, err := code.PNG(size)
	if err != nil {
		return nil, fmt.Errorf("could not render qr png: %w", err)
	}

	return png, nil
}

func parseRecoveryLevel(level string) (qrcode.RecoveryLevel, error) {
	switch level {
	case "", "M":
		return qrcode.Medium, nil
	case "L":
		return qrcode.Low, nil
	case "Q":
		return qrcode.High, nil
	case "H":
		return qrcode.Highest, nil
	default:
		return 0, fmt.Errorf("invalid recovery level: %q (expected L, M, Q, or H)", level)
	}
}

func parseHexColor(hexStr string) (color.RGBA, error) {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) != 6 {
		return color.RGBA{}, fmt.Errorf("expected #rrggbb format, got %q", hexStr)
	}

	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}, nil
}
