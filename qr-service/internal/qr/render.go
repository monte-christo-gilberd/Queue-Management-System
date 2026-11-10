package qr

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	// defaultSize: lebar gambar bawaan dalam piksel (512).
	defaultSize = 512
	// minSize: lebar minimal yang diizinkan (128 piksel).
	minSize = 128
	// maxSize: lebar maksimal yang diizinkan (2048 piksel).
	maxSize = 2048
)

// Options: parameter render QR. Nilai kosong atau nol memakai bawaan.
type Options struct {
	// Data: isi yang dikodekan ke QR.
	Data string
	// Size: lebar gambar dalam piksel. Dibatasi ke rentang minSize sampai maxSize.
	Size int
	// RecoveryLevel: tingkat koreksi error: L, M, Q, atau H. Kosong = M.
	RecoveryLevel string
	// ForegroundColor: warna depan #rrggbb. Kosong = warna bawaan library.
	ForegroundColor string
	// BackgroundColor: warna latar #rrggbb. Kosong = warna bawaan library.
	BackgroundColor string
}

// Render draws opts.Data as a PNG QR code, applying sane defaults/bounds for
// size and recovery level and optional #rrggbb foreground/background colors.
func Render(opts Options) ([]byte, error) {
	// Mulai dari ukuran dari opsi.
	size := opts.Size
	// Ukuran tidak diisi (nol atau negatif): pakai bawaan.
	if size <= 0 {
		size = defaultSize
	}
	// Di bawah batas minimal: naikkan ke minSize.
	if size < minSize {
		size = minSize
	}
	// Di atas batas maksimal: turunkan ke maxSize.
	if size > maxSize {
		size = maxSize
	}

	// Terjemahkan teks tingkat koreksi (L, M, Q, H) ke tipe library. Teks tidak dikenal menghasilkan galat.
	level, err := parseRecoveryLevel(opts.RecoveryLevel)
	if err != nil {
		return nil, err
	}

	// Buat kode QR dari data dengan tingkat koreksi tadi. Data terlalu panjang untuk tingkat ini menghasilkan galat.
	code, err := qrcode.New(opts.Data, level)
	if err != nil {
		return nil, fmt.Errorf("could not build qr code: %w", err)
	}

	// Warna depan diisi: ubah dari #rrggbb lalu pasang ke kode QR.
	if opts.ForegroundColor != "" {
		fg, err := parseHexColor(opts.ForegroundColor)
		// Format warna salah: tolak dengan membungkus galat asli.
		if err != nil {
			return nil, fmt.Errorf("invalid foreground color: %w", err)
		}
		code.ForegroundColor = fg
	}

	// Warna latar diisi: ubah dari #rrggbb lalu pasang ke kode QR.
	if opts.BackgroundColor != "" {
		bg, err := parseHexColor(opts.BackgroundColor)
		// Format warna salah: tolak dengan membungkus galat asli.
		if err != nil {
			return nil, fmt.Errorf("invalid background color: %w", err)
		}
		code.BackgroundColor = bg
	}

	// Gambar kode QR menjadi PNG dengan lebar yang sudah dibatasi.
	png, err := code.PNG(size)
	// Gagal menggambar PNG: bungkus galatnya dan kembalikan.
	if err != nil {
		return nil, fmt.Errorf("could not render qr png: %w", err)
	}

	return png, nil
}

// parseRecoveryLevel mengubah teks tingkat koreksi (L, M, Q, H) menjadi tipe library. Kosong dan M memakai Medium.
func parseRecoveryLevel(level string) (qrcode.RecoveryLevel, error) {
	// Pilih tingkat sesuai teks. Kosong dan M sama-sama Medium.
	switch level {
	// Kosong dan M: tingkat menengah (bawaan).
	case "", "M":
		return qrcode.Medium, nil
	// L: koreksi rendah.
	case "L":
		return qrcode.Low, nil
	// Q: koreksi tinggi (Quartile).
	case "Q":
		return qrcode.High, nil
	// H: koreksi tertinggi.
	case "H":
		return qrcode.Highest, nil
	// Teks tidak dikenal: tolak dan sebutkan nilai yang diizinkan.
	default:
		return 0, fmt.Errorf("invalid recovery level: %q (expected L, M, Q, or H)", level)
	}
}

// parseHexColor mengubah warna #rrggbb (boleh tanpa #) menjadi color.RGBA dengan alpha penuh.
func parseHexColor(hexStr string) (color.RGBA, error) {
	// Buang tanda # di depan bila ada.
	hexStr = strings.TrimPrefix(hexStr, "#")
	// Panjang bukan enam karakter: format salah, tolak.
	if len(hexStr) != 6 {
		return color.RGBA{}, fmt.Errorf("expected #rrggbb format, got %q", hexStr)
	}

	// Decode hex menjadi tiga byte (merah, hijau, biru). Karakter bukan hex menghasilkan galat.
	decoded, err := hex.DecodeString(hexStr)
	// Decode gagal: kembalikan galatnya.
	if err != nil {
		return color.RGBA{}, err
	}

	// Susun warna dengan alpha 255 (tidak transparan).
	return color.RGBA{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}, nil
}
