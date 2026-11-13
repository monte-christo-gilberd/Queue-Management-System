package model

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// JoinCodeLength: panjang kode join yang diketik manual atau dibaca dari QR (karakter).
const JoinCodeLength = 6

// joinCodeAlphabet: huruf dan angka untuk kode join. Karakter yang mudah tertukar (0/O, 1/I/L) dibuang
// supaya kode mudah dibaca dan diketik. 32 karakter, sehingga ada sekitar satu miliar kombinasi.
const joinCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// NewJoinCode membuat kode join acak dengan crypto/rand.
func NewJoinCode() (string, error) {
	// Batas acak = jumlah karakter di alfabet, sehingga setiap karakter punya peluang yang sama.
	max := big.NewInt(int64(len(joinCodeAlphabet)))
	// Siapkan buffer hasil sepanjang JoinCodeLength.
	out := make([]byte, JoinCodeLength)
	for i := range out {
		// Ambil indeks acak dari crypto/rand; gagal berarti sumber acak bermasalah, kembalikan error.
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		// Ganti posisi ke-i dengan karakter dari alfabet pada indeks tersebut.
		out[i] = joinCodeAlphabet[n.Int64()]
	}
	// Semua karakter terisi: ubah buffer menjadi string kode.
	return string(out), nil
}

// NormalizeJoinCode merapikan kode yang diketik pengguna: spasi di tepi dibuang dan huruf dijadikan kapital.
// Mengembalikan false bila panjang atau karakternya tidak sesuai kode join.
func NormalizeJoinCode(raw string) (string, bool) {
	// Buang spasi di tepi dan jadikan huruf kapital supaya huruf kecil juga diterima.
	code := strings.ToUpper(strings.TrimSpace(raw))
	// Panjang harus tepat JoinCodeLength karakter.
	if len(code) != JoinCodeLength {
		return "", false
	}
	// Setiap karakter harus ada di alfabet kode; karakter seperti 0, O, 1, I, L langsung ditolak.
	for i := 0; i < len(code); i++ {
		if strings.IndexByte(joinCodeAlphabet, code[i]) < 0 {
			return "", false
		}
	}
	// Kode valid: kembalikan bentuk yang sudah dirapikan.
	return code, true
}
