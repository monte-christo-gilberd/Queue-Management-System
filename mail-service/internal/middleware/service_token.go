package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// ServiceToken membungkus handler dan menolak request yang X-Service-Token-nya tidak cocok dengan token layanan
// (perbandingan dalam waktu tetap).
func ServiceToken(
	token string,
) func(http.HandlerFunc) http.HandlerFunc {
	// kembalikan pembungkus yang bisa dipasang ke handler mana pun
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Ambil token dari header request.
			got := r.Header.Get("X-Service-Token")
			// Token tidak cocok: tolak dengan 401 dan jangan jalankan handler. Perbandingan berwaktu tetap supaya
			// lama respons tidak membocorkan seberapa banyak awal token yang benar.
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				abortError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
				return
			}
			// Token cocok: jalankan handler berikutnya.
			next(w, r)
		}
	}
}

// abortError menulis respons error JSON yang seragam dengan handler, lalu request dihentikan oleh pemanggilnya.
func abortError(
	w http.ResponseWriter,
	status int,
	code string,
	msg string,
) {
	// Tandai respons sebagai JSON dan larang cache.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Tulis status, lalu body error standar. Galat encode diabaikan karena header sudah terkirim.
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
