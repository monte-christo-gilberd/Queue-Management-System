package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qms/qr-service/internal/joinlink"
	"qms/qr-service/internal/qr"
)

const (
	// maxGenericDataLength: panjang maksimal data yang bisa dirender menjadi QR (1024 karakter).
	maxGenericDataLength = 1024
	// minCustomTTL: batas bawah TTL yang diminta klien untuk QR dinamis (30 detik).
	minCustomTTL = 30 * time.Second
	// maxCustomTTL: batas atas TTL yang diminta klien untuk QR dinamis (10 menit).
	maxCustomTTL = 10 * time.Minute
)

// errorBody: isi error di dalam respons: kode tetap dan pesan yang aman ditampilkan.
type errorBody struct {
	// Code: kode error tetap, mis. BAD_REQUEST, QR_EXPIRED, atau UNAUTHORIZED.
	Code string `json:"code"`
	// Message: penjelasan singkat untuk klien, tanpa detail teknis internal.
	Message string `json:"message"`
}

// errorResponse: bentuk respons error yang dikirim semua endpoint.
type errorResponse struct {
	// Error: isi error (kode dan pesan).
	Error errorBody `json:"error"`
}

// Handlers memegang dependensi endpoint publik dan internal.
type Handlers struct {
	// signer: pembuat dan verifikator token join.
	signer *joinlink.Signer
	// dynamicTTL: masa berlaku bawaan token dinamis.
	dynamicTTL time.Duration
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
}

// NewHandlers membuat Handlers dengan signer dan TTL dinamis. Waktu memakai time.Now.
func NewHandlers(
	signer *joinlink.Signer,
	dynamicTTL time.Duration,
) *Handlers {
	// Susun Handlers dengan dependensi dari argumen dan time.Now sebagai sumber waktu.
	return &Handlers{
		signer:     signer,
		dynamicTTL: dynamicTTL,
		now:        time.Now,
	}
}

// Health menjawab liveness probe.
func (h *Handlers) Health(c *gin.Context) {
	// Balas 200 dengan status ok agar orkestrasi tahu proses masih hidup.
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready menjawab readiness probe. Konfigurasi sudah divalidasi saat start, jadi
// service yang berjalan selalu siap melayani.
func (h *Handlers) Ready(c *gin.Context) {
	// Balas 200 dengan status ready; service sudah siap karena konfigurasi divalidasi saat start.
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// issueTokenRequest: body untuk membuat token join.
type issueTokenRequest struct {
	// QueueID: id antrian yang akan dibuatkan token. Harus positif.
	QueueID int64 `json:"queue_id"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// TTLSeconds: masa berlaku yang diminta dalam detik, hanya untuk dynamic. 0 = pakai bawaan.
	TTLSeconds int `json:"ttl_seconds"`
}

// tokenResponse: token join yang dibuat.
type tokenResponse struct {
	// Token: string token join yang siap dipakai di tautan QR.
	Token string `json:"token"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix. Nil untuk token statis.
	ExpiresAt *int64 `json:"expires_at"`
}

// IssueToken membuat token join baru. Dipakai backend saat membuat QR statis
// pertama kali atau saat meminta token dinamis.
func (h *Handlers) IssueToken(c *gin.Context) {
	// Baca dan decode body JSON ke issueTokenRequest.
	var req issueTokenRequest
	// Body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400.
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// queue_id tidak positif: tolak.
	if req.QueueID <= 0 {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "queue_id must be positive")
		return
	}

	// Ubah teks jenis token menjadi tipe joinlink.Type.
	typ := joinlink.Type(req.Type)
	// Jenis token tidak dikenal (bukan static atau dynamic): balas 400 dengan kode QR_TYPE_INVALID.
	if !typ.Valid() {
		abortWithError(c, http.StatusBadRequest, "QR_TYPE_INVALID", "type must be static or dynamic")
		return
	}

	// Mulai dari TTL bawaan layanan.
	ttl := h.dynamicTTL
	// Token dinamis dan klien mengirim TTL sendiri: pakai TTL klien setelah memeriksa rentangnya.
	if typ == joinlink.TypeDynamic && req.TTLSeconds > 0 {
		// Ubah detik dari klien menjadi durasi.
		ttl = time.Duration(req.TTLSeconds) * time.Second
		// TTL di luar rentang yang diizinkan: tolak.
		if ttl < minCustomTTL || ttl > maxCustomTTL {
			abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "ttl_seconds out of range")
			return
		}
	}

	// Buat dan tandatangani token. Galat di sini dianggap permintaan yang tidak bisa dipenuhi.
	token, claims, err := h.signer.Issue(req.QueueID, typ, ttl)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "could not issue token")
		return
	}

	// Susun respons dari token dan jenisnya.
	resp := tokenResponse{Token: token, Type: string(claims.Type)}
	// Token punya waktu kedaluwarsa (dinamis): salin ke pointer agar statis tetap null.
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	// Kirim token dengan status 200.
	c.JSON(http.StatusOK, resp)
}

// verifyTokenRequest: body untuk memeriksa token join.
type verifyTokenRequest struct {
	// Token: token join yang akan diperiksa.
	Token string `json:"token"`
}

// verifyTokenResponse: hasil pemeriksaan token join yang valid.
type verifyTokenResponse struct {
	// Valid: selalu true pada respons sukses; respons galat dibalas dengan kode error.
	Valid bool `json:"valid"`
	// QueueID: id antrian yang tertulis di dalam token.
	QueueID int64 `json:"queue_id"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix. Nil untuk token statis.
	ExpiresAt *int64 `json:"expires_at"`
}

// VerifyToken memeriksa tanda tangan dan masa berlaku token. Backend tetap wajib
// mencocokkan token statis dengan yang tersimpan di queue_settings.
func (h *Handlers) VerifyToken(c *gin.Context) {
	// Baca dan decode body JSON ke verifyTokenRequest.
	var req verifyTokenRequest
	// Body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400.
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// Periksa format, tanda tangan, dan masa berlaku token. Galat diterjemahkan ke kode HTTP di bawah.
	claims, err := h.signer.Verify(req.Token)
	if err != nil {
		abortForTokenError(c, err)
		return
	}

	// Token valid: susun respons dari klaim token.
	resp := verifyTokenResponse{
		Valid:   true,
		QueueID: claims.QueueID,
		Type:    string(claims.Type),
	}
	// Token punya waktu kedaluwarsa (dinamis): salin ke pointer agar statis tetap null.
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	// Kirim hasil verifikasi dengan status 200.
	c.JSON(http.StatusOK, resp)
}

// renderRequest: body untuk merender QR umum menjadi PNG.
type renderRequest struct {
	// Data: isi yang dikodekan ke QR. Wajib diisi, maksimal 1024 karakter.
	Data string `json:"data"`
	// Size: lebar gambar dalam piksel. 0 = bawaan 512; nilai di luar 128 sampai 2048 dibatasi ke rentang itu.
	Size int `json:"size"`
	// Level: tingkat koreksi error: L, M, Q, atau H. Kosong = M.
	Level string `json:"level"`
	// FG: warna depan dalam format #rrggbb. Kosong = hitam.
	FG string `json:"fg"`
	// BG: warna latar dalam format #rrggbb. Kosong = putih.
	BG string `json:"bg"`
}

// RenderGeneric merender QR apa pun untuk kebutuhan internal lain (mis. QR
// pembayaran). Tidak menerima data rahasia dari klien publik.
func (h *Handlers) RenderGeneric(c *gin.Context) {
	// Baca dan decode body JSON ke renderRequest.
	var req renderRequest
	// Body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400.
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// Data kosong atau melebihi 1024 karakter: tolak.
	if req.Data == "" || len(req.Data) > maxGenericDataLength {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "data is required (max 1024 characters)")
		return
	}

	// Render data menjadi PNG dengan opsi dari request. Galat (level, warna, atau data) dianggap permintaan salah.
	png, err := qr.Render(qr.Options{
		Data:            req.Data,
		Size:            req.Size,
		RecoveryLevel:   req.Level,
		ForegroundColor: req.FG,
		BackgroundColor: req.BG,
	})
	// Render gagal: balas 400 dengan pesan galat dari renderer.
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// Jangan simpan gambar di cache, lalu kirim PNG ke klien.
	c.Header("Cache-Control", "no-store")
	writePNG(c, png)
}

// writePNG mengirim bytes PNG dengan header Content-Type yang benar dan nosniff.
func writePNG(c *gin.Context, png []byte) {
	// Tandai isi sebagai PNG dan larang browser menebak tipe lain.
	c.Header("Content-Type", "image/png")
	c.Header("X-Content-Type-Options", "nosniff")
	// Kirim bytes PNG dengan status 200.
	c.Data(http.StatusOK, "image/png", png)
}

// abortForTokenError menerjemahkan galat verifikasi token ke kode HTTP dan kode error yang tetap.
func abortForTokenError(c *gin.Context, err error) {
	// Pilih respons sesuai jenis galat. Galat yang tidak dikenal jatuh ke default.
	switch {
	// Token sudah kedaluwarsa: 410 Gone.
	case errors.Is(err, joinlink.ErrExpired):
		abortWithError(c, http.StatusGone, "QR_EXPIRED", "token expired")
	// Tanda tangan tidak cocok: token dipalsukan atau secret salah. 401.
	case errors.Is(err, joinlink.ErrInvalidSignature):
		abortWithError(c, http.StatusUnauthorized, "QR_INVALID", "invalid token")
	// Format, versi, atau isi payload salah: 400 dengan kode QR_MALFORMED.
	case errors.Is(err, joinlink.ErrMalformed),
		errors.Is(err, joinlink.ErrUnsupportedFormat),
		errors.Is(err, joinlink.ErrInvalidPayload):
		abortWithError(c, http.StatusBadRequest, "QR_MALFORMED", "malformed token")
	// Galat lain yang tidak dikenal: anggap token tidak valid, tanpa membocorkan detail.
	default:
		abortWithError(c, http.StatusBadRequest, "QR_MALFORMED", "invalid token")
	}
}

// abortWithError menghentikan request dan mengirim respons error dalam bentuk errorResponse.
func abortWithError(c *gin.Context, status int, code, message string) {
	// Tulis status dan body JSON error, lalu hentikan rantai handler.
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}
