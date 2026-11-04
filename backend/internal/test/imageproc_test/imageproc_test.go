package imageproc_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"strings"
	"testing"

	"qms/internal/imageproc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noisy: gambar berpola supaya hasil JPEG tidak mengecil secara trivial.
func noisy(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7), uint8(y * 13), uint8((x ^ y) * 5), 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}))
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err, "hasil harus JPEG valid")
	return img
}

func TestProcess_JPEGLandscapeBecomesCroppedSquare512(t *testing.T) {
	out, err := imageproc.Process(bytes.NewReader(encodeJPEG(t, noisy(1600, 900))))
	require.NoError(t, err)
	b := decode(t, out).Bounds()
	assert.Equal(t, imageproc.OutputSize, b.Dx())
	assert.Equal(t, imageproc.OutputSize, b.Dy())
	assert.LessOrEqual(t, len(out), imageproc.MaxOutputBytes)
}

func TestProcess_PNGPortraitAndSmallImageNotUpscaled(t *testing.T) {
	out, err := imageproc.Process(bytes.NewReader(encodePNG(t, noisy(300, 700))))
	require.NoError(t, err)
	b := decode(t, out).Bounds()
	assert.Equal(t, 300, b.Dx(), "foto kecil tidak diperbesar; sisi = sisi terpendek")
	assert.Equal(t, 300, b.Dy())
}

func TestProcess_TransparentPNGBecomesWhite(t *testing.T) {
	out, err := imageproc.Process(bytes.NewReader(encodePNG(t, image.NewRGBA(image.Rect(0, 0, 64, 64)))))
	require.NoError(t, err)
	r, g, b, _ := decode(t, out).At(32, 32).RGBA()
	assert.Greater(t, r>>8, uint32(240))
	assert.Greater(t, g>>8, uint32(240))
	assert.Greater(t, b>>8, uint32(240))
}

func TestProcess_WebP(t *testing.T) {
	// WebP lossless 1x1 (fixture kecil; stdlib tidak bisa meng-encode WebP).
	raw, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	require.NoError(t, err)
	out, err := imageproc.Process(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Equal(t, 1, decode(t, out).Bounds().Dx())
}

func TestProcess_DropsEXIFAndOtherMetadata(t *testing.T) {
	jpg := encodeJPEG(t, noisy(200, 200))
	// Sisipkan segmen APP1 "Exif" palsu tepat setelah SOI.
	exif := append([]byte{0xFF, 0xE1, 0x00, 0x10}, []byte("Exif\x00\x00GPS-LOKASI!")...)
	tagged := append(append(append([]byte{}, jpg[:2]...), exif...), jpg[2:]...)
	require.True(t, bytes.Contains(tagged, []byte("GPS-LOKASI!")))

	out, err := imageproc.Process(bytes.NewReader(tagged))
	require.NoError(t, err)
	assert.False(t, bytes.Contains(out, []byte("GPS-LOKASI!")))
	assert.False(t, bytes.Contains(out, []byte("Exif")))
}

func TestProcess_RejectsUnsupportedByContentNotName(t *testing.T) {
	var g bytes.Buffer
	require.NoError(t, gif.Encode(&g, noisy(10, 10), nil))
	for name, data := range map[string][]byte{
		"gif":    g.Bytes(),
		"svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":   []byte("<html><body>x</body></html>"),
		"teks":   []byte("ini bukan gambar"),
		"kosong": {},
		"pdf":    []byte("%PDF-1.4 ..."),
	} {
		_, err := imageproc.Process(bytes.NewReader(data))
		assert.ErrorIs(t, err, imageproc.ErrUnsupported, name)
	}
}

func TestProcess_RejectsCorruptImage(t *testing.T) {
	jpg := encodeJPEG(t, noisy(100, 100))
	_, err := imageproc.Process(bytes.NewReader(jpg[:len(jpg)/3]))
	assert.ErrorIs(t, err, imageproc.ErrInvalid)

	_, err = imageproc.Process(bytes.NewReader(append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, []byte("sampah")...)))
	assert.ErrorIs(t, err, imageproc.ErrInvalid)
}

func TestProcess_RejectsHugeDimensionsWithoutDecoding(t *testing.T) {
	// PNG kecil yang mengaku 10000x10000 (100 megapiksel): ditolak dari header.
	full := encodePNG(t, image.NewGray(image.Rect(0, 0, 8, 8)))
	// Ubah lebar dan tinggi di chunk IHDR (offset 16..23); CRC tidak diperiksa oleh DecodeConfig.
	bomb := append([]byte{}, full...)
	copy(bomb[16:20], []byte{0x00, 0x00, 0x27, 0x10})
	copy(bomb[20:24], []byte{0x00, 0x00, 0x27, 0x10})
	_, err := imageproc.Process(bytes.NewReader(bomb))
	assert.ErrorIs(t, err, imageproc.ErrInvalid)
}

func TestProcess_RejectsOverFiveMegabytes(t *testing.T) {
	big := append(encodeJPEG(t, noisy(50, 50)), bytes.Repeat([]byte{0}, imageproc.MaxUploadBytes)...)
	_, err := imageproc.Process(bytes.NewReader(big))
	assert.ErrorIs(t, err, imageproc.ErrTooLarge)

	_, err = imageproc.Process(strings.NewReader(strings.Repeat("a", imageproc.MaxUploadBytes+1)))
	assert.ErrorIs(t, err, imageproc.ErrTooLarge)
}

func TestProcess_NoisyLargePhotoStaysUnderOutputLimit(t *testing.T) {
	out, err := imageproc.Process(bytes.NewReader(encodePNG(t, noisy(2000, 2000))))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), imageproc.MaxOutputBytes)
}

func TestProcess_RandomNoiseWorstCaseStillFits(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.Intn(256))
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	out, err := imageproc.Process(bytes.NewReader(encodePNG(t, img)))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), imageproc.MaxOutputBytes)
}
