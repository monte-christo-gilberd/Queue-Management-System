package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestID memberi setiap request id untuk korelasi log. Nilai dari header
// X-Request-ID dipakai bila ada (mis. diteruskan backend), selain itu dibuat baru.
func RequestID() gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// ambil request ID dari header yang diteruskan backend
		id := c.GetHeader("X-Request-ID")
		// header kosong atau terlalu panjang (di atas 128 karakter): buat ID baru
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		// simpan ID di konteks agar Logger bisa mencatatnya
		c.Set("request_id", id)
		// kirim ID kembali ke klien untuk korelasi
		c.Header("X-Request-ID", id)
		// lanjutkan ke handler berikutnya
		c.Next()
	}
}

// newRequestID membuat ID acak 12 byte dalam bentuk hex (24 karakter). Bila sumber acak gagal, kembalikan "unknown".
func newRequestID() string {
	// siapkan buffer 12 byte acak
	buf := make([]byte, 12)
	// isi dengan byte acak dari crypto/rand; bila gagal, pakai nilai penanda "unknown"
	if _, err := rand.Read(buf); err != nil {
		// tanpa sumber acak, pakai penanda tetap agar request tetap berjalan
		return "unknown"
	}
	// ubah ke hex agar aman ditulis di header dan log
	return hex.EncodeToString(buf)
}
