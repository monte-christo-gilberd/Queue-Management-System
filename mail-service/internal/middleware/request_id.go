package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"qms/mail-service/internal/requestid"
)

// maxRequestIDLen: panjang maksimum request ID dari luar. Lebih panjang dari ini dianggap tidak sah.
const maxRequestIDLen = 128

// RequestID memberi setiap request id untuk korelasi log. Nilai dari header X-Request-Id (diisi nginx) dipakai bila
// ada dan bentuknya wajar, selain itu dibuat baru. ID disimpan di context Gin (kunci "request_id"), di context
// request (untuk lapisan bawah yang butuh korelasi log), dan dikirim kembali di header respons.
func RequestID() gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// ambil request ID dari header yang diteruskan nginx
		id := c.GetHeader("X-Request-Id")
		// header kosong, terlalu panjang, atau berisi karakter aneh: buat ID baru agar log tidak bisa disisipi isi liar
		if !validRequestID(id) {
			id = newRequestID()
		}
		// simpan ID di konteks Gin agar Logger dan Recovery bisa mencatatnya
		c.Set("request_id", id)
		// simpan juga di context request supaya lapisan bawah bisa membaca ID yang sama
		c.Request = c.Request.WithContext(requestid.With(c.Request.Context(), id))
		// catat ID sebagai atribut span (no-op bila tidak ada span), supaya trace bisa dicari lewat request_id
		trace.SpanFromContext(c.Request.Context()).SetAttributes(attribute.String("request_id", id))
		// kirim ID kembali ke klien untuk korelasi
		c.Header("X-Request-Id", id)
		// lanjutkan ke handler berikutnya
		c.Next()
	}
}

// validRequestID menerima ID tidak kosong, sampai 128 karakter, hanya huruf, angka, "-", "_", dan ".".
func validRequestID(
	id string,
) bool {
	// kosong atau terlalu panjang: tidak sah
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	// periksa tiap karakter terhadap daftar yang diizinkan
	for _, r := range id {
		// huruf, angka, dan tanda baca aman di header maupun log
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		// satu karakter di luar daftar sudah cukup untuk menolak
		if !ok {
			return false
		}
	}
	// semua karakter lolos
	return true
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
