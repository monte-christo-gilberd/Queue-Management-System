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
	SendVerification(ctx context.Context, userID int64, requestIP string) error
	Verify(ctx context.Context, token string) error
}

type EmailVerificationHandler struct {
	service EmailVerificationService
	log     *slog.Logger
}

func NewEmailVerificationHandler(service EmailVerificationService) *EmailVerificationHandler {
	return &EmailVerificationHandler{service: service, log: slog.Default()}
}

// Verify: POST /api/v1/users/verify-email (publik; kredensialnya token di tautan email). Token tidak sah,
// kedaluwarsa, atau sudah dipakai: 400 INVALID_TOKEN (satu pesan untuk ketiganya).
func (h *EmailVerificationHandler) Verify(c *gin.Context) {
	var req dto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	err := h.service.Verify(c.Request.Context(), req.Token)
	switch {
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "verification link is invalid, expired or already used")
	case err != nil:
		h.log.Error("verify email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}

// Resend: POST /api/v1/users/me/verify-email/resend (butuh login). 202 bila email dikirim atau akun sudah
// terverifikasi (tidak ada yang dikirim). Batas 3 per jam per akun: 429 TOO_MANY_REQUESTS.
func (h *EmailVerificationHandler) Resend(c *gin.Context) {
	err := h.service.SendVerification(c.Request.Context(), c.GetInt64("user_id"), c.ClientIP())
	switch {
	case errors.Is(err, apperror.ErrTooManyVerifyEmails):
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", err.Error())
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
	case err != nil:
		h.log.Error("resend verification email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	default:
		respondData(c, http.StatusAccepted, gin.H{"message": "Jika email belum terverifikasi, tautan verifikasi sudah dikirim."})
	}
}
