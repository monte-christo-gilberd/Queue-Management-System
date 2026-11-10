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
	// ErrTooLarge: berkas unggahan melebihi MaxUploadBytes.
	ErrTooLarge = errors.New("image file too large")
	// ErrUnsupported: isi berkas bukan JPEG, PNG, atau WebP.
	ErrUnsupported = errors.New("unsupported image type")
	// ErrInvalid: berkas rusak, dimensinya tidak wajar, atau gagal diolah.
	ErrInvalid = errors.New("invalid image")
)

// qualities dicoba berurutan sampai hasil muat di MaxOutputBytes.
var qualities = []int{82, 72, 62, 50, 40, 30}

// Process membaca unggahan (maks MaxUploadBytes) dan mengembalikan JPEG persegi. Jenis ditentukan dari isi
// berkas, bukan dari nama atau header yang dikirim klien. Yang diterima: JPEG, PNG, WebP.
func Process(r io.Reader) ([]byte, error) {
	// baca maksimal MaxUploadBytes+1 byte; +1 dipakai untuk mendeteksi berkas yang melebihi batas
	raw, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	// gagal membaca dari klien: anggap berkas tidak valid
	if err != nil {
		return nil, ErrInvalid
	}
	// hasil baca lebih dari batas berarti berkas terlalu besar
	if len(raw) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	// deteksi jenis dari isi byte-nya (bukan dari nama berkas)
	switch http.DetectContentType(raw) {
	// jenis yang diterima: tidak perlu tindakan, lanjut ke pengolahan
	case "image/jpeg", "image/png", "image/webp":
	// jenis lain ditolak
	default:
		return nil, ErrUnsupported
	}

	// Ukuran dibaca dari header dulu; decode penuh hanya bila dimensinya wajar.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	// header gagal dibaca, atau lebar/tinggi tidak masuk akal (nol atau melebihi maxPixels): tolak
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrInvalid
	}
	// decode gambar penuh untuk diolah; gagal berarti isi berkas rusak
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalid
	}

	// ambil batas (kotak) gambar sumber
	b := src.Bounds()
	// sisi persegi diambil dari sisi terpendek agar hasil crop tidak gepeng
	side := min(b.Dx(), b.Dy())
	// hitung kotak crop persegi yang ada di tengah gambar
	crop := image.Rect(
		b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2,
		b.Min.X+(b.Dx()-side)/2+side, b.Min.Y+(b.Dy()-side)/2+side,
	)
	// ukuran keluaran: OutputSize, tetapi bila foto lebih kecil, pakai ukuran aslinya (tidak diperbesar)
	out := min(side, OutputSize)
	// kanvas tujuan berukuran out x out
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	// JPEG tidak punya alpha: bagian transparan PNG/WebP jadi putih.
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	// salin area crop dari gambar sumber ke kanvas, diperkecil dengan algoritma CatmullRom
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Over, nil)

	// coba setiap tingkat kualitas JPEG dari yang tertinggi; berhenti di yang pertama muat di batas ukuran
	for _, q := range qualities {
		// buffer untuk menampung hasil encode pada kualitas q
		var buf bytes.Buffer
		// encode kanvas ke JPEG dengan kualitas q; gagal berarti hasil tidak bisa dibuat
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
			return nil, ErrInvalid
		}
		// hasil sudah sesuai batas ukuran: kembalikan bytes JPEG-nya
		if buf.Len() <= MaxOutputBytes {
			return buf.Bytes(), nil
		}
	}
	// semua kualitas masih terlalu besar: anggap gagal
	return nil, ErrInvalid
}
