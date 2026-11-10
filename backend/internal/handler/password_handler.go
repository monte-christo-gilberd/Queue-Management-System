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

// PasswordResetService adalah logika lupa dan reset password (dipenuhi oleh service).
type PasswordResetService interface {
	// RequestReset membuat token reset dan mengirim tautannya ke email, bila email terdaftar.
	RequestReset(ctx context.Context, email, requestIP string) error
	// ResetPassword memakai token reset untuk mengganti password lama dengan newPassword.
	ResetPassword(ctx context.Context, token, newPassword string) error
}

const (
	// Pengiriman email berjalan di goroutine (respons seragam, tidak membocorkan ada-tidaknya akun lewat waktu).
	// Batas ini mencegah goroutine menumpuk bila mail-service macet.
	maxResetInFlight = 32
	// resetJobTimeout adalah batas waktu satu pengiriman email reset di goroutine.
	resetJobTimeout = 45 * time.Second

	// forgotPasswordMessage adalah pesan netral yang sama untuk email terdaftar maupun tidak.
	forgotPasswordMessage = "Jika email terdaftar, tautan untuk membuat password baru sudah dikirim."
)

// PasswordHandler menangani rute lupa dan reset password.
type PasswordHandler struct {
	// service berisi logika reset password.
	service PasswordResetService
	// sem membatasi jumlah pengiriman email yang sedang berjalan (semafor berkapasitas maxResetInFlight).
	sem chan struct{}
	// log mencatat error agar bisa ditelusuri, tanpa alamat email dan tanpa diteruskan ke klien.
	log *slog.Logger
}

// NewPasswordHandler membuat PasswordHandler dengan service, semafor, dan logger bawaan.
func NewPasswordHandler(service PasswordResetService) *PasswordHandler {
	// isi handler dengan semafor kosong berkapasitas maxResetInFlight
	return &PasswordHandler{service: service, sem: make(chan struct{}, maxResetInFlight), log: slog.Default()}
}

// ForgotPassword: POST /api/v1/users/forgot-password. Selalu 202 dengan pesan netral untuk email yang formatnya sah,
// terdaftar atau tidak. error pengiriman hanya dicatat di log (tanpa alamat email) dan tidak sampai ke klien.
func (h *PasswordHandler) ForgotPassword(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// alamat email harus berformat sah; jika tidak, tolak
	if _, err := mail.ParseAddress(req.Email); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid email")
		return
	}

	// ambil satu slot dari semafor; bila semua slot terpakai, tolak dengan 503 agar tidak menumpuk
	select {
	case h.sem <- struct{}{}:
	default:
		respondError(c, http.StatusServiceUnavailable, "BUSY", "too many requests, try again shortly")
		return
	}
	// simpan IP klien sebelum masuk goroutine, karena context request bisa selesai lebih dulu
	ip := c.ClientIP()
	// kirim email di goroutine agar respons tetap cepat dan seragam
	go func() {
		// lepaskan slot semafor saat pekerjaan selesai
		defer func() { <-h.sem }()
		// beri batas waktu sendiri, tidak mengikuti context request yang sudah selesai
		ctx, cancel := context.WithTimeout(context.Background(), resetJobTimeout)
		defer cancel()
		// error hanya dicatat di log, tidak dikirim ke klien
		if err := h.service.RequestReset(ctx, req.Email, ip); err != nil {
			h.log.Error("password reset request failed", "err", err)
		}
	}()

	// respons seragam: 202 dengan pesan netral, terlepas dari hasil pengiriman
	respondData(c, http.StatusAccepted, gin.H{"message": forgotPasswordMessage})
}

// ResetPassword: POST /api/v1/users/reset-password. Token tidak sah, kedaluwarsa, atau sudah dipakai: 400
// INVALID_TOKEN (satu pesan untuk ketiganya).
func (h *PasswordHandler) ResetPassword(c *gin.Context) {
	// baca body JSON yang berisi token dan password baru; gagal berarti format request salah
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// ganti password memakai token; pilih respons sesuai hasilnya
	err := h.service.ResetPassword(c.Request.Context(), req.Token, req.Password)
	switch {
	// password baru tidak memenuhi panjang 8 sampai 72 karakter
	case errors.Is(err, apperror.ErrWeakPassword):
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	// token tidak sah, kedaluwarsa, atau sudah dipakai
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "reset link is invalid, expired or already used")
	// error lain: catat di log dan balas 500 tanpa detail
	case err != nil:
		h.log.Error("password reset failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil: tandai respons jangan di-cache, lalu balas ok
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}
