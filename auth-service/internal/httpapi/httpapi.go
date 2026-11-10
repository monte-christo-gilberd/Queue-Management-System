// Package httpapi: rute HTTP auth-service. Path publik dan bentuk respons sama dengan yang dulu dilayani backend
// (/api/v1/users/...), jadi frontend tidak berubah. nginx meneruskan path ini ke layanan ini.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/auth"
)

const (
	// maxResetInFlight membatasi pengiriman email reset yang berjalan di goroutine (mail-service bisa macet).
	maxResetInFlight = 32
	resetJobTimeout  = 45 * time.Second
	// forgotPasswordMessage: pesan netral yang sama untuk email terdaftar maupun tidak.
	forgotPasswordMessage = "Jika email terdaftar, tautan untuk membuat password baru sudah dikirim."
)

// RegisterRequest: body POST /users/register.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,no_html_tags"`
	Username string `json:"username" binding:"required,no_html_tags"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest: body login. Menerima `identifier` (email atau username); `email` tetap diterima sebagai alias lama.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"omitempty,no_html_tags"`
	Email      string `json:"email" binding:"omitempty,no_html_tags"`
	Password   string `json:"password" binding:"required"`
}

// Login mengembalikan identifier yang dipakai: `identifier` jika ada, kalau tidak `email`.
func (r LoginRequest) Login() string {
	if r.Identifier != "" {
		return r.Identifier
	}
	return r.Email
}

// AuthResponse: isi `data` respons login dan registrasi.
type AuthResponse struct {
	Token string `json:"token"`
}

// ForgotPasswordRequest: body POST /users/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,max=254,no_html_tags"`
}

// ResetPasswordRequest: body POST /users/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required,max=200"`
	Password string `json:"password" binding:"required"`
}

// VerifyEmailRequest: body POST /users/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required,max=200"`
}

// RegisterValidators memasang aturan `no_html_tags` (menolak '<' dan '>') ke validator bawaan gin.
func RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("no_html_tags", func(fl validator.FieldLevel) bool {
			return !strings.ContainsAny(fl.Field().String(), "<>")
		})
	}
}

// Handler menangani semua rute auth-service.
type Handler struct {
	// svc: logika akun.
	svc *auth.Service
	// sem: semafor yang membatasi pengiriman email reset yang sedang berjalan (kapasitas maxResetInFlight).
	sem chan struct{}
	// log: mencatat galat server tanpa alamat email dan tanpa diteruskan ke klien.
	log *slog.Logger
}

// NewRouter membuat engine gin. emailOn = mail-service terpasang: hanya dengan itu reset password dan verifikasi
// email didaftarkan (sama seperti perilaku backend sebelumnya).
func NewRouter(svc *auth.Service, emailOn bool, trustedProxies []string) *gin.Engine {
	// gin.New tanpa logger bawaan: log akses dicatat nginx; Recovery mengubah panic menjadi 500
	engine := gin.New()
	engine.Use(gin.Recovery())
	// hanya proxy ini yang boleh menentukan IP klien lewat X-Forwarded-For (dipakai pembatas per akun)
	_ = engine.SetTrustedProxies(trustedProxies)
	h := &Handler{svc: svc, sem: make(chan struct{}, maxResetInFlight), log: slog.Default()}

	// Liveness: tidak menyentuh database, sehingga gangguan DB tidak membuat layanan dianggap mati.
	engine.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// rute publik: register dan login selalu ada
	users := engine.Group("/api/v1/users")
	users.POST("/register", h.Register)
	users.POST("/login", h.Login)
	// reset password dan verifikasi email butuh mail-service; tanpa itu rutenya tidak ada (404)
	if emailOn {
		users.POST("/forgot-password", h.ForgotPassword)
		users.POST("/reset-password", h.ResetPassword)
		users.POST("/verify-email", h.VerifyEmail)
	}

	// rute yang butuh login: hanya kirim ulang verifikasi (profil dan lainnya tetap di backend)
	me := engine.Group("/api/v1/users/me")
	me.Use(h.requireAuth)
	if emailOn {
		me.POST("/verify-email/resend", h.ResendVerification)
	}
	return engine
}

// respondData menulis respons sukses {"data": ...}.
func respondData(c *gin.Context, status int, data any) { c.JSON(status, gin.H{"data": data}) }

// respondError menulis respons galat {"error": {"code", "message"}}. code adalah kode tetap yang dibaca klien.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// requireAuth memeriksa JWT RS256 dan pencabutan sesi, lalu menyimpan user_id di context.
func (h *Handler) requireAuth(c *gin.Context) {
	// baca header Authorization; tanpa awalan "Bearer " request ditolak
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		c.Abort()
		return
	}
	// periksa token dan pencabutan sesi; galat dipetakan ke status di bawah
	userID, err := h.svc.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
	switch {
	// token tidak sah, kedaluwarsa, atau sesi dicabut: 401
	case errors.Is(err, apperror.ErrUnauthenticated):
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		c.Abort()
		return
	// gangguan database: 500, bukan salah klien
	case err != nil:
		h.log.Error("authenticate failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		c.Abort()
		return
	}
	// token sah: simpan id user di context agar handler bisa membacanya, lalu teruskan request
	c.Set("user_id", userID)
	c.Next()
}

// Register: POST /api/v1/users/register.
func (h *Handler) Register(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// buat akun; galat dipetakan ke respons di bawah
	token, err := h.svc.Register(c.Request.Context(), req.Email, req.Username, req.Password)
	switch {
	// password kurang dari 8 atau lebih dari 72: 400
	case errors.Is(err, apperror.ErrWeakPassword):
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	// email sudah dipakai: 409
	case errors.Is(err, apperror.ErrDuplicate):
		respondError(c, http.StatusConflict, "CONFLICT", "conflict")
	// galat lain: catat di log, balas 500 tanpa detail
	case err != nil:
		h.log.Error("register failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil: kirim token login
	default:
		respondData(c, http.StatusOK, AuthResponse{Token: token})
	}
}

// Login: POST /api/v1/users/login. Semua kegagalan dibalas sama (401) agar tidak membocorkan akun yang ada.
func (h *Handler) Login(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// identifier dari `identifier` atau alias `email`; kosong berarti tidak ada yang diisi
	identifier := req.Login()
	if identifier == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "identifier (email or username) is required")
		return
	}
	// periksa kredensial; kegagalan kredensial tidak dicatat, hanya galat tak terduga (mis. database)
	token, err := h.svc.Login(c.Request.Context(), identifier, req.Password)
	if err != nil {
		if !errors.Is(err, apperror.ErrNotFound) {
			h.log.Error("login failed", "err", err)
		}
		// balasan sama untuk semua kegagalan agar tidak membocorkan akun yang ada
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}
	// kredensial benar: kirim token login
	respondData(c, http.StatusOK, AuthResponse{Token: token})
}

// ForgotPassword: POST /api/v1/users/forgot-password. Selalu 202 dengan pesan netral untuk email yang formatnya sah.
func (h *Handler) ForgotPassword(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// alamat email harus berformat sah
	if _, err := mail.ParseAddress(req.Email); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid email")
		return
	}
	// ambil satu slot semafor; bila penuh, tolak dengan 503 agar goroutine tidak menumpuk saat mail-service macet
	select {
	case h.sem <- struct{}{}:
	default:
		respondError(c, http.StatusServiceUnavailable, "BUSY", "too many requests, try again shortly")
		return
	}
	// simpan IP klien sebelum masuk goroutine, karena context request bisa selesai lebih dulu
	ip := c.ClientIP()
	// kirim di goroutine agar respons cepat dan seragam (tidak membocorkan ada-tidaknya akun lewat waktu)
	go func() {
		// lepaskan slot saat pekerjaan selesai
		defer func() { <-h.sem }()
		// batas waktu sendiri, tidak mengikuti context request
		ctx, cancel := context.WithTimeout(context.Background(), resetJobTimeout)
		defer cancel()
		// galat hanya dicatat (tanpa alamat email), tidak dikirim ke klien
		if err := h.svc.RequestReset(ctx, req.Email, ip); err != nil {
			h.log.Error("password reset request failed", "err", err)
		}
	}()
	// respons seragam: 202 dengan pesan netral, apa pun hasil pengirimannya
	respondData(c, http.StatusAccepted, gin.H{"message": forgotPasswordMessage})
}

// ResetPassword: POST /api/v1/users/reset-password.
func (h *Handler) ResetPassword(c *gin.Context) {
	// baca body JSON berisi token dan password baru
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// ganti password memakai token; pilih respons sesuai hasil
	err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password)
	switch {
	// password baru di luar 8 sampai 72 karakter
	case errors.Is(err, apperror.ErrWeakPassword):
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	// token tidak sah, kedaluwarsa, atau sudah dipakai: satu pesan untuk ketiganya
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "reset link is invalid, expired or already used")
	// galat lain: catat, balas 500 tanpa detail
	case err != nil:
		h.log.Error("password reset failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil: jangan di-cache
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}

// VerifyEmail: POST /api/v1/users/verify-email (publik; kredensialnya token di tautan email).
func (h *Handler) VerifyEmail(c *gin.Context) {
	// baca body JSON berisi token dari tautan email
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// jalankan verifikasi lalu pilih respons sesuai hasil
	err := h.svc.VerifyEmail(c.Request.Context(), req.Token)
	switch {
	// token tidak sah, kedaluwarsa, atau sudah dipakai: satu pesan untuk ketiganya
	case errors.Is(err, apperror.ErrInvalidResetToken):
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "verification link is invalid, expired or already used")
	// galat lain: catat, balas 500 tanpa detail
	case err != nil:
		h.log.Error("verify email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil: jangan di-cache
	default:
		c.Header("Cache-Control", "no-store")
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	}
}

// ResendVerification: POST /api/v1/users/me/verify-email/resend (butuh login). Batas 3 per jam per akun: 429.
func (h *Handler) ResendVerification(c *gin.Context) {
	// kirim ulang untuk user yang login, dengan IP klien untuk pembatasan
	err := h.svc.SendVerification(c.Request.Context(), c.GetInt64("user_id"), c.ClientIP())
	switch {
	// batas 3 per jam habis: 429
	case errors.Is(err, apperror.ErrTooManyVerifyEmails):
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", err.Error())
	// akun sudah tidak ada: sesi dianggap tidak sah
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
	// galat lain: catat, balas 500 tanpa detail
	case err != nil:
		h.log.Error("resend verification email failed", "err", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	// berhasil atau tidak perlu dikirim: 202 dengan pesan netral
	default:
		respondData(c, http.StatusAccepted, gin.H{"message": "Jika email belum terverifikasi, tautan verifikasi sudah dikirim."})
	}
}
