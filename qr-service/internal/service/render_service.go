package service

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"qms/qr-service/internal/apperror"
	"qms/qr-service/internal/model"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	// maxGenericDataLength: panjang maksimal data yang bisa dirender menjadi QR (1024 karakter).
	maxGenericDataLength = 1024
	// defaultSize: lebar gambar bawaan dalam piksel (512).
	defaultSize = 512
	// minSize: lebar minimal yang diizinkan (128 piksel).
	minSize = 128
	// maxSize: lebar maksimal yang diizinkan (2048 piksel).
	maxSize = 2048
)

// RenderService merender kode QR menjadi PNG. Tidak mengetahui apa pun tentang antrian.
type RenderService struct{}

// NewRenderService membuat RenderService. Service ini tidak punya dependency.
func NewRenderService() *RenderService {
	// service tanpa state, cukup nilai kosong
	return &RenderService{}
}

// Render menggambar opts.Data sebagai PNG QR, dengan batas ukuran, tingkat koreksi, dan warna #rrggbb opsional.
func (s *RenderService) Render(
	opts model.RenderOptions,
) ([]byte, error) {
	// data kosong atau melebihi 1024 karakter: tolak
	if opts.Data == "" || len(opts.Data) > maxGenericDataLength {
		return nil, apperror.ErrInvalidRenderData
	}

	// mulai dari ukuran dari opsi
	size := opts.Size
	// ukuran tidak diisi (nol atau negatif): pakai bawaan
	if size <= 0 {
		// tanpa ukuran dari klien: pakai bawaan
		size = defaultSize
	}
	// di bawah batas minimal: naikkan ke minSize
	if size < minSize {
		// ukuran dinaikkan ke batas bawah, bukan ditolak
		size = minSize
	}
	// di atas batas maksimal: turunkan ke maxSize
	if size > maxSize {
		// ukuran diturunkan ke batas atas, bukan ditolak
		size = maxSize
	}

	// terjemahkan teks tingkat koreksi (L, M, Q, H) ke tipe library; teks tidak dikenal menghasilkan galat
	level, err := parseRecoveryLevel(opts.RecoveryLevel)
	// tingkat koreksi tidak dikenal: teruskan galat ke pemanggil
	if err != nil {
		return nil, err
	}

	// buat kode QR dari data dengan tingkat koreksi tadi; data terlalu panjang untuk tingkat ini menghasilkan galat
	code, err := qrcode.New(opts.Data, level)
	// data terlalu panjang untuk tingkat koreksi ini: bungkus galat library
	if err != nil {
		return nil, fmt.Errorf("could not build qr code: %w", err)
	}

	// warna depan diisi: ubah dari #rrggbb lalu pasang ke kode QR
	if opts.ForegroundColor != "" {
		fg, err := parseHexColor(opts.ForegroundColor)
		// format warna salah: tolak dengan membungkus galat asli
		if err != nil {
			return nil, fmt.Errorf("invalid foreground color: %w", err)
		}
		// pasang warna depan
		code.ForegroundColor = fg
	}

	// warna latar diisi: ubah dari #rrggbb lalu pasang ke kode QR
	if opts.BackgroundColor != "" {
		bg, err := parseHexColor(opts.BackgroundColor)
		// format warna salah: tolak dengan membungkus galat asli
		if err != nil {
			return nil, fmt.Errorf("invalid background color: %w", err)
		}
		// pasang warna latar
		code.BackgroundColor = bg
	}

	// gambar kode QR menjadi PNG dengan lebar yang sudah dibatasi
	png, err := code.PNG(size)
	// gagal menggambar PNG: bungkus galatnya dan kembalikan
	if err != nil {
		return nil, fmt.Errorf("could not render qr png: %w", err)
	}

	// kembalikan bytes PNG
	return png, nil
}

// parseRecoveryLevel mengubah teks tingkat koreksi (L, M, Q, H) menjadi tipe library. Kosong dan M memakai Medium.
func parseRecoveryLevel(
	level string,
) (qrcode.RecoveryLevel, error) {
	// pilih tingkat sesuai teks; kosong dan M sama-sama Medium
	switch level {
	// kosong dan M: tingkat menengah (bawaan)
	case "", "M":
		// Medium memulihkan sekitar 15% data rusak
		return qrcode.Medium, nil
	// L: koreksi rendah
	case "L":
		// Low memulihkan sekitar 7% data rusak
		return qrcode.Low, nil
	// Q: koreksi tinggi (Quartile)
	case "Q":
		// High memulihkan sekitar 25% data rusak
		return qrcode.High, nil
	// H: koreksi tertinggi
	case "H":
		// Highest memulihkan sekitar 30% data rusak
		return qrcode.Highest, nil
	// teks tidak dikenal: tolak dan sebutkan nilai yang diizinkan
	default:
		// sebut nilai yang diizinkan agar klien bisa memperbaiki input
		return 0, fmt.Errorf("invalid recovery level: %q (expected L, M, Q, or H)", level)
	}
}

// parseHexColor mengubah warna #rrggbb (boleh tanpa #) menjadi color.RGBA dengan alpha penuh.
func parseHexColor(
	hexStr string,
) (color.RGBA, error) {
	// buang tanda # di depan bila ada
	hexStr = strings.TrimPrefix(hexStr, "#")
	// panjang bukan enam karakter: format salah, tolak
	if len(hexStr) != 6 {
		return color.RGBA{}, fmt.Errorf("expected #rrggbb format, got %q", hexStr)
	}

	// decode hex menjadi tiga byte (merah, hijau, biru); karakter bukan hex menghasilkan galat
	decoded, err := hex.DecodeString(hexStr)
	// decode gagal: kembalikan galatnya
	if err != nil {
		return color.RGBA{}, err
	}

	// susun warna dengan alpha 255 (tidak transparan)
	return color.RGBA{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}, nil
}
