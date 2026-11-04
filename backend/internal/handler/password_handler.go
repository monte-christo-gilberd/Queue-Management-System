package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"qms/internal/apperror"
	"qms/internal/dto"
	"time"

	"github.com/gin-gonic/gin"
)

type PasswordResetService interface {
	RequestReset(ctx context.Context, email, requestIP string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

const (
	// Pengiriman email berjalan di goroutine (respons seragam, tidak membocorkan ada-tidaknya akun lewat waktu).
	// Batas ini mencegah goroutine menumpuk bila mail-service macet.
	maxResetInFlight = 32
	resetJobTimeout  = 45 * time.Second

	forgotPasswordMessage = "Jika email terdaftar, tautan untuk membuat password baru sudah dikirim."
)

type PasswordHandler struct {
	service PasswordResetService
	sem     chan struct{}
	log     *slog.Logger
}

func NewPasswordHandler(service PasswordResetService) *PasswordHandler {
	return &PasswordHandler{service: service, sem: make(chan struct{}, maxResetInFlight), log: slog.Default()}
}

// ForgotPassword: POST /api/v1/users/forgot-password. Selalu 202 dengan pesan netral untuk email yang formatnya sah,
// terdaftar atau tidak. Galat pengiriman hanya dicatat di log (tanpa alamat email) dan tidak sampai ke klien.
func (h *PasswordHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid email")
		return
	}

	select {
	case h.sem <- struct{}{}:
	default:
		respondError(c, http.StatusServiceUnavailable, "BUSY", "too many requests, try again shortly")
		return
	}
	ip := c.ClientIP()
	go func() {
		defer func() { <-h.sem }()
		ctx, cancel := context.WithTimeout(context.Background(), resetJobTimeout)
		defer cancel()
		if err := h.service.RequestReset(ctx, req.Email, ip); err != nil {
			h.log.Error("password reset request failed", "err", err)
		}
	}()

	respondData(c, http.StatusAccepted, gin.H{"message": forgotPasswordMessage})
}

// ResetPassword: POST /api/v1/users/reset-password. Token tidak sah, kedaluwarsa, atau sudah dipakai: 400
// INVALID_TOKEN (satu pesan untuk ketiganya).
func (h *PasswordHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	err := h.service.ResetPassword(c.Request.Context(), req.Token, req.Password)
	switch {
	case errors.Is(err, apperror.ErrWeakPassword):
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "reset link is invalid, expired or already used")
	case err != nil:
		h.log.Error("password reset failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}
