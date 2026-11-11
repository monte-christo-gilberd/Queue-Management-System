package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// sign menghitung HMAC-SHA256 dari signingInput dengan secret, lalu mengodekannya ke base64url tanpa padding.
func (s *TokenService) sign(
	signingInput string,
) string {
	// buat HMAC dengan SHA-256 dan secret milik service
	mac := hmac.New(sha256.New, s.secret)
	// masukkan teks yang ditandatangani ke HMAC
	mac.Write([]byte(signingInput))
	// kodekan hasil tanda tangan ke base64url tanpa padding agar aman di URL
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// randomID membuat id acak 16 byte dalam base64url tanpa padding. Dipakai sebagai JTI.
func randomID() (string, error) {
	// siapkan buffer 16 byte acak
	buf := make([]byte, 16)
	// isi dengan byte acak dari crypto/rand; galat dikembalikan ke pemanggil
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// kodekan ke base64url tanpa padding
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
