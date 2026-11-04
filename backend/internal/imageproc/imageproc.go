// Package imageproc mengolah foto profil dari unggahan user: validasi isi berkas, crop persegi, perkecil, dan
// encode ulang ke JPEG. Encode ulang membuang EXIF dan metadata lain (termasuk lokasi GPS).
package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // mendaftarkan decoder PNG untuk image.Decode
	"io"
	"net/http"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // mendaftarkan decoder WebP untuk image.Decode
)

const (
	// MaxUploadBytes: batas ukuran berkas unggahan.
	MaxUploadBytes = 5 << 20
	// MaxOutputBytes: batas hasil olahan; sama dengan CHECK di tabel user_avatars.
	MaxOutputBytes = 150 << 10
	// OutputSize: sisi gambar hasil, persegi.
	OutputSize = 512
	// maxPixels mencegah decompression bomb: berkas kecil yang mengembang jadi gambar raksasa di memori.
	maxPixels = 24_000_000
)

var (
	ErrTooLarge    = errors.New("image file too large")
	ErrUnsupported = errors.New("unsupported image type")
	ErrInvalid     = errors.New("invalid image")
)

// qualities dicoba berurutan sampai hasil muat di MaxOutputBytes.
var qualities = []int{82, 72, 62, 50, 40, 30}

// Process membaca unggahan (maks MaxUploadBytes) dan mengembalikan JPEG persegi. Jenis ditentukan dari isi
// berkas, bukan dari nama atau header yang dikirim klien. Yang diterima: JPEG, PNG, WebP.
func Process(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return nil, ErrInvalid
	}
	if len(raw) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, ErrUnsupported
	}

	// Ukuran dibaca dari header dulu; decode penuh hanya bila dimensinya wajar.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrInvalid
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalid
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(
		b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2,
		b.Min.X+(b.Dx()-side)/2+side, b.Min.Y+(b.Dy()-side)/2+side,
	)
	out := min(side, OutputSize) // foto kecil tidak diperbesar
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	// JPEG tidak punya alpha: bagian transparan PNG/WebP jadi putih.
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Over, nil)

	for _, q := range qualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
			return nil, ErrInvalid
		}
		if buf.Len() <= MaxOutputBytes {
			return buf.Bytes(), nil
		}
	}
	return nil, ErrInvalid
}
