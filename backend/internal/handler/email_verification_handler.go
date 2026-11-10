package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"

	"github.com/gin-gonic/gin"
)

// EmailVerificationService: dipenuhi service.EmailVerificationService.
type EmailVerificationService interface {
	// SendVerification mengirim email verifikasi ke user, dengan batas pengiriman per akun.
	SendVerification(ctx context.Context, userID int64, requestIP string) error
	// Verify memeriksa token dari tautan email dan menandai email sebagai terverifikasi.
	Verify(ctx context.Context, token string) error
}

// EmailVerificationHandler menangani rute verifikasi email: verifikasi lewat tautan dan kirim ulang.
type EmailVerificationHandler struct {
	// service berisi logika verifikasi email.
	service EmailVerificationService
	// log mencatat error server agar bisa ditelusuri, tanpa diteruskan ke klien.
	log *slog.Logger
}

// NewEmailVerificationHandler membuat EmailVerificationHandler dengan service dan logger bawaan.
func NewEmailVerificationHandler(service EmailVerificationService) *EmailVerificationHandler {
	// isi handler dengan service dan logger bawaan
	return &EmailVerificationHandler{service: service, log: slog.Default()}
}

// Verify: POST /api/v1/users/verify-email (publik; kredensialnya token di tautan email). Token tidak sah,
// kedaluwarsa, atau sudah dipakai: 400 INVALID_TOKEN (satu pesan untuk ketiganya).
func (h *EmailVerificationHandler) Verify(c *gin.Context) {
	// baca body JSON yang berisi token; gagal berarti format request salah
	var req dto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// jalankan verifikasi token lalu pilih respons sesuai hasilnya
	err := h.service.Verify(c.Request.Context(), req.Token)
	switch {
	// token tidak sah, kedaluwarsa, atau sudah dipakai: 400 dengan pesan yang sama untuk ketiganya
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "verification link is invalid, expired or already used")
	// error lain: catat di log dan balas 500 tanpa detail
	case err != nil:
		h.log.Error("verify email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil: tandai respons jangan di-cache, lalu balas ok
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}

// Resend: POST /api/v1/users/me/verify-email/resend (butuh login). 202 bila email dikirim atau akun sudah
// terverifikasi (tidak ada yang dikirim). Batas 3 per jam per akun: 429 TOO_MANY_REQUESTS.
func (h *EmailVerificationHandler) Resend(c *gin.Context) {
	// kirim ulang email verifikasi untuk user yang login, dengan IP klien untuk pembatasan
	err := h.service.SendVerification(c.Request.Context(), c.GetInt64("user_id"), c.ClientIP())
	switch {
	// batas pengiriman per jam sudah habis: 429
	case errors.Is(err, apperror.ErrTooManyVerifyEmails):
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", err.Error())
	// akun tidak ditemukan: anggap sesi tidak sah
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
	// error lain: catat di log dan balas 500 tanpa detail
	case err != nil:
		h.log.Error("resend verification email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil atau tidak perlu dikirim: 202 dengan pesan netral
	default:
		respondData(c, http.StatusAccepted, gin.H{"message": "Jika email belum terverifikasi, tautan verifikasi sudah dikirim."})
	}
}
