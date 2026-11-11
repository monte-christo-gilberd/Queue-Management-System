package handler

import (
	"errors"
	"net/http"
	"qms/qr-service/internal/apperror"
	"qms/qr-service/internal/dto"
	"qms/qr-service/internal/model"

	"github.com/gin-gonic/gin"
)

// TokenService adalah logika token join (dipenuhi oleh service).
type TokenService interface {
	// IssueToken memeriksa aturan input lalu membuat token join baru.
	IssueToken(
		queueID int64,
		typ string,
		ttlSeconds int,
	) (string, model.Claims, error)
	// VerifyToken memeriksa format, tanda tangan, dan masa berlaku token.
	VerifyToken(
		token string,
	) (model.Claims, error)
}

// TokenHandler menangani rute pembuatan dan verifikasi token join.
type TokenHandler struct {
	// service berisi logika token.
	service TokenService
}

// NewTokenHandler membuat TokenHandler dengan service yang diberikan.
func NewTokenHandler(
	service TokenService,
) *TokenHandler {
	// isi handler dengan service
	return &TokenHandler{
		service: service,
	}
}

// IssueToken membuat token join baru. Dipakai backend saat membuat QR statis
// pertama kali atau saat meminta token dinamis.
func (h *TokenHandler) IssueToken(c *gin.Context) {
	// baca dan decode body JSON ke IssueTokenRequest
	var req dto.IssueTokenRequest
	// body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400
	if err := c.ShouldBindJSON(&req); err != nil {
		// 400 dengan kode umum; detail kesalahan JSON tidak dibocorkan ke klien
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// minta service memeriksa aturan input dan membuat token
	token, claims, err := h.service.IssueToken(req.QueueID, req.Type, req.TTLSeconds)
	// gagal: terjemahkan error service ke kode HTTP
	if err != nil {
		// pilih status dan kode error sesuai jenis galat service
		switch {
		// queue_id tidak positif: 400 BAD_REQUEST
		case errors.Is(err, apperror.ErrInvalidQueueID):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", "queue_id must be positive")
		// jenis token tidak dikenal: 400 QR_TYPE_INVALID
		case errors.Is(err, apperror.ErrInvalidTokenType):
			respondError(c, http.StatusBadRequest, "QR_TYPE_INVALID", "type must be static or dynamic")
		// TTL di luar rentang: 400 BAD_REQUEST
		case errors.Is(err, apperror.ErrTTLOutOfRange):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", "ttl_seconds out of range")
		// galat lain saat membuat token dianggap permintaan yang tidak bisa dipenuhi
		default:
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", "could not issue token")
		}
		// respons error sudah ditulis, jangan lanjut membuat respons sukses
		return
	}

	// susun respons dari token dan jenisnya
	resp := dto.TokenResponse{Token: token, Type: string(claims.Type)}
	// token punya waktu kedaluwarsa (dinamis): salin ke pointer agar statis tetap null
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	// kirim token dengan status 200
	respondJSON(c, resp)
}

// VerifyToken memeriksa tanda tangan dan masa berlaku token. Backend tetap wajib
// mencocokkan token statis dengan yang tersimpan di queue_settings.
func (h *TokenHandler) VerifyToken(c *gin.Context) {
	// baca dan decode body JSON ke VerifyTokenRequest
	var req dto.VerifyTokenRequest
	// body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400
	if err := c.ShouldBindJSON(&req); err != nil {
		// 400 dengan kode umum; detail kesalahan JSON tidak dibocorkan ke klien
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// periksa format, tanda tangan, dan masa berlaku token
	claims, err := h.service.VerifyToken(req.Token)
	// gagal: terjemahkan error service ke kode HTTP
	if err != nil {
		// respondTokenError memilih 410, 401, atau 400 sesuai galat
		respondTokenError(c, err)
		return
	}

	// token valid: susun respons dari klaim token
	resp := dto.VerifyTokenResponse{
		Valid:   true,
		QueueID: claims.QueueID,
		Type:    string(claims.Type),
	}
	// token punya waktu kedaluwarsa (dinamis): salin ke pointer agar statis tetap null
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	// kirim hasil verifikasi dengan status 200
	respondJSON(c, resp)
}

// respondTokenError menerjemahkan galat verifikasi token ke kode HTTP dan kode error yang tetap.
func respondTokenError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis galat; galat yang tidak dikenal jatuh ke default
	switch {
	// token sudah kedaluwarsa: 410 Gone
	case errors.Is(err, apperror.ErrTokenExpired):
		// 410 Gone: klien perlu meminta QR baru
		respondError(c, http.StatusGone, "QR_EXPIRED", "token expired")
	// tanda tangan tidak cocok: token dipalsukan atau secret salah, 401
	case errors.Is(err, apperror.ErrTokenInvalidSignature):
		// 401: tanda tangan salah berarti token bukan berasal dari layanan ini
		respondError(c, http.StatusUnauthorized, "QR_INVALID", "invalid token")
	// format, versi, atau isi payload salah: 400 dengan kode QR_MALFORMED
	case errors.Is(err, apperror.ErrTokenMalformed),
		errors.Is(err, apperror.ErrTokenUnsupportedFormat),
		errors.Is(err, apperror.ErrInvalidPayload):
		respondError(c, http.StatusBadRequest, "QR_MALFORMED", "malformed token")
	// galat lain yang tidak dikenal: anggap token tidak valid, tanpa membocorkan detail
	default:
		// pesan umum agar detail galat internal tidak bocor
		respondError(c, http.StatusBadRequest, "QR_MALFORMED", "invalid token")
	}
}
