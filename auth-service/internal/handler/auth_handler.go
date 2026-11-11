package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"time"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/dto"
	"qms/auth-service/internal/service"
)

const (
	// maxResetInFlight membatasi pengiriman email reset yang berjalan di goroutine (mail-service bisa macet).
	maxResetInFlight = 32
	// resetJobTimeout: batas waktu satu pengiriman email reset di goroutine.
	resetJobTimeout = 45 * time.Second
	// forgotPasswordMessage: pesan netral yang sama untuk email terdaftar maupun tidak.
	forgotPasswordMessage = "Jika email terdaftar, tautan untuk membuat password baru sudah dikirim."
	// resendVerificationMessage: pesan netral untuk kirim ulang verifikasi.
	resendVerificationMessage = "Jika email belum terverifikasi, tautan verifikasi sudah dikirim."
)

// AuthService adalah logika akun (dipenuhi oleh service.Service): registrasi, login, reset password,
// verifikasi email, dan login Google.
type AuthService interface {
	// Register membuat akun dan mengembalikan token login.
	Register(
		ctx context.Context,
		email,
		username,
		password string,
	) (string, error)
	// Login memeriksa email atau username beserta password, lalu mengembalikan token login.
	Login(
		ctx context.Context,
		identifier,
		password string,
	) (string, error)
	// RequestReset membuat token reset dan mengirim tautannya ke email akun.
	RequestReset(
		ctx context.Context,
		email,
		requestIP string,
	) error
	// ResetPassword memasang password baru bila token sah.
	ResetPassword(
		ctx context.Context,
		token,
		newPassword string,
	) error
	// SendVerification membuat token verifikasi dan mengirim tautannya.
	SendVerification(
		ctx context.Context,
		userID int64,
		requestIP string,
	) error
	// VerifyEmail menandai email terverifikasi bila token sah.
	VerifyEmail(
		ctx context.Context,
		token string,
	) error
	// LoginWithGoogle menukar ID token Google dengan token login.
	LoginWithGoogle(
		ctx context.Context,
		idToken string,
	) (string, error)
	// LinkGoogle menautkan akun Google setelah user membuktikan password akunnya.
	LinkGoogle(
		ctx context.Context,
		linkToken,
		password string,
	) (string, error)
}

// AuthHandler menangani semua rute akun auth-service.
type AuthHandler struct {
	// svc: logika akun.
	svc AuthService
	// sem: semafor yang membatasi pengiriman email reset yang sedang berjalan (kapasitas maxResetInFlight).
	sem chan struct{}
	// log: mencatat galat server tanpa alamat email dan tanpa diteruskan ke klien.
	log *slog.Logger
}

// NewAuthHandler membuat AuthHandler dengan service yang diberikan.
func NewAuthHandler(
	svc AuthService,
) *AuthHandler {
	// isi handler dengan service, semafor kosong, dan logger bawaan
	return &AuthHandler{
		svc: svc,
		sem: make(chan struct{}, maxResetInFlight),
		log: slog.Default(),
	}
}

// Register: POST /api/v1/users/register.
func (h *AuthHandler) Register(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.RegisterRequest
	// body JSON tidak sah ditolak sebelum diteruskan ke service
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST; pesan generik tanpa detail parser
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// buat akun; galat diterjemahkan ke respons HTTP
	token, err := h.svc.Register(c.Request.Context(), req.Email, req.Username, req.Password)
	// cek hasil pendaftaran
	if err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondRegisterError(c, err)
		return
	}
	// berhasil: kirim token login
	respondData(c, http.StatusOK, dto.AuthResponse{Token: token})
}

// respondRegisterError menerjemahkan error Register menjadi respons HTTP.
func (h *AuthHandler) respondRegisterError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error
	switch {
	// password kurang dari 8 atau lebih dari 72: 400
	case errors.Is(err, apperror.ErrWeakPassword):
		// kirim 400 WEAK_PASSWORD agar klien menampilkan aturan panjang password
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	// email sudah dipakai: 409
	case errors.Is(err, apperror.ErrDuplicate):
		// kirim 409 CONFLICT dengan pesan generik
		respondError(c, http.StatusConflict, "CONFLICT", "conflict")
	// galat lain: catat di log, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("register failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// Login: POST /api/v1/users/login. Semua kegagalan dibalas sama (401) agar tidak membocorkan akun yang ada.
func (h *AuthHandler) Login(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.LoginRequest
	// body JSON tidak sah ditolak sebelum login diproses
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// identifier dari `identifier` atau alias `email`; kosong berarti tidak ada yang diisi
	identifier := req.Login()
	// login butuh email atau username; keduanya kosong ditolak
	if identifier == "" {
		// balas 400 dengan petunjuk field yang kurang
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "identifier (email or username) is required")
		return
	}
	// periksa kredensial; galat diterjemahkan ke respons HTTP
	token, err := h.svc.Login(c.Request.Context(), identifier, req.Password)
	// cek hasil login
	if err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondLoginError(c, err)
		return
	}
	// kredensial benar: kirim token login
	respondData(c, http.StatusOK, dto.AuthResponse{Token: token})
}

// respondLoginError membalas semua kegagalan login dengan 401 yang sama; hanya galat tak terduga yang dicatat.
func (h *AuthHandler) respondLoginError(
	c *gin.Context,
	err error,
) {
	// kegagalan kredensial tidak dicatat, hanya galat tak terduga (mis. database)
	if !errors.Is(err, apperror.ErrNotFound) {
		// catat galat tak terduga di log server
		h.log.Error("login failed", "err", err)
	}
	// balasan sama untuk semua kegagalan agar tidak membocorkan akun yang ada
	respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
}

// ForgotPassword: POST /api/v1/users/forgot-password. Selalu 202 dengan pesan netral untuk email yang formatnya sah.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.ForgotPasswordRequest
	// body JSON tidak sah ditolak sebelum proses reset
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// alamat email harus berformat sah
	if _, err := mail.ParseAddress(req.Email); err != nil {
		// balas 400 untuk format email tidak sah
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid email")
		return
	}
	// ambil satu slot semafor; bila penuh, tolak dengan 503 agar goroutine tidak menumpuk saat mail-service macet
	select {
	// slot tersedia: lanjut mengirim
	case h.sem <- struct{}{}:
	// semafor penuh: tolak, jangan menumpuk goroutine
	default:
		// balas 503 BUSY agar klien mencoba lagi nanti
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
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	// baca body JSON berisi token dan password baru
	var req dto.ResetPasswordRequest
	// body JSON tidak sah ditolak sebelum token diperiksa
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// ganti password memakai token; galat diterjemahkan ke respons HTTP
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondResetError(c, err)
		return
	}
	// berhasil: jangan di-cache
	c.Header("Cache-Control", "no-store")
	// balas 200 dengan status ok
	respondData(c, http.StatusOK, gin.H{"status": "ok"})
}

// respondResetError menerjemahkan error ResetPassword menjadi respons HTTP.
func (h *AuthHandler) respondResetError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error
	switch {
	// password baru di luar 8 sampai 72 karakter
	case errors.Is(err, apperror.ErrWeakPassword):
		// kirim 400 WEAK_PASSWORD
		respondError(c, http.StatusBadRequest, "WEAK_PASSWORD", "password must be 8 to 72 characters")
	// token tidak sah, kedaluwarsa, atau sudah dipakai: satu pesan untuk ketiganya
	case errors.Is(err, apperror.ErrInvalidResetToken):
		// kirim 400 INVALID_TOKEN; satu pesan agar tidak membocorkan keadaan token
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "reset link is invalid, expired or already used")
	// galat lain: catat, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("password reset failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// VerifyEmail: POST /api/v1/users/verify-email (publik; kredensialnya token di tautan email).
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	// baca body JSON berisi token dari tautan email
	var req dto.VerifyEmailRequest
	// body JSON tidak sah ditolak sebelum token diperiksa
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// jalankan verifikasi; galat diterjemahkan ke respons HTTP
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondVerifyError(c, err)
		return
	}
	// berhasil: jangan di-cache
	c.Header("Cache-Control", "no-store")
	// balas 200 dengan status ok
	respondData(c, http.StatusOK, gin.H{"status": "ok"})
}

// respondVerifyError menerjemahkan error VerifyEmail menjadi respons HTTP.
func (h *AuthHandler) respondVerifyError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error
	switch {
	// token tidak sah, kedaluwarsa, atau sudah dipakai: satu pesan untuk ketiganya
	case errors.Is(err, apperror.ErrInvalidResetToken):
		// kirim 400 INVALID_TOKEN
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "verification link is invalid, expired or already used")
	// galat lain: catat, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("verify email failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// ResendVerification: POST /api/v1/users/me/verify-email/resend (butuh login). Batas 3 per jam per akun: 429.
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	// kirim ulang untuk user yang login, dengan IP klien untuk pembatasan; galat diterjemahkan ke respons HTTP
	err := h.svc.SendVerification(c.Request.Context(), c.GetInt64("user_id"), c.ClientIP())
	// cek hasil pengiriman ulang
	if err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondResendError(c, err)
		return
	}
	// berhasil atau tidak perlu dikirim: 202 dengan pesan netral
	respondData(c, http.StatusAccepted, gin.H{"message": resendVerificationMessage})
}

// respondResendError menerjemahkan error SendVerification menjadi respons HTTP.
func (h *AuthHandler) respondResendError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error
	switch {
	// batas 3 per jam habis: 429
	case errors.Is(err, apperror.ErrTooManyVerifyEmails):
		// kirim 429 dengan pesan error bawaan (coba lagi nanti)
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", err.Error())
	// akun sudah tidak ada: sesi dianggap tidak sah
	case errors.Is(err, apperror.ErrNotFound):
		// kirim 401 seperti login gagal
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
	// galat lain: catat, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("resend verification email failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// GoogleLogin: POST /api/v1/users/google. Menukar ID token Google dengan token login QMS.
//   - 200 {data:{token}}: akun sudah tertaut, atau akun baru dibuat;
//   - 409 GOOGLE_LINK_REQUIRED + error.link_token: email sudah terdaftar dengan password, perlu konfirmasi password;
//   - 401 UNAUTHORIZED: ID token tidak sah; 403 GOOGLE_EMAIL_UNVERIFIED: email belum diverifikasi Google;
//   - 503 GOOGLE_UNAVAILABLE: kunci Google tidak bisa diambil.
//
// ID token dan link token tidak pernah dicatat di log.
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	// respons memuat token: jangan di-cache
	c.Header("Cache-Control", "no-store")
	// baca body JSON; gagal berarti format request salah
	var req dto.GoogleLoginRequest
	// body JSON tidak sah ditolak sebelum ID token diperiksa
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// tukar ID token dengan token login; galat diterjemahkan ke respons HTTP
	token, err := h.svc.LoginWithGoogle(c.Request.Context(), req.IDToken)
	// cek hasil login Google
	if err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondGoogleLoginError(c, err)
		return
	}
	// berhasil: kirim token login
	respondData(c, http.StatusOK, dto.AuthResponse{Token: token})
}

// respondGoogleLoginError menerjemahkan error LoginWithGoogle menjadi respons HTTP.
func (h *AuthHandler) respondGoogleLoginError(
	c *gin.Context,
	err error,
) {
	// wadah untuk error yang membawa link token
	var link *service.LinkRequiredError
	// pilih respons sesuai jenis error
	switch {
	// email sudah terdaftar dengan password: kirim link token untuk layar konfirmasi password
	case errors.As(err, &link):
		// balas 409 dengan kode GOOGLE_LINK_REQUIRED dan link token
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":       "GOOGLE_LINK_REQUIRED",
			"message":    "this email is already registered, confirm your password to link your Google account",
			"link_token": link.Token,
		}})
	// ID token tidak sah (tanda tangan, issuer, audience, atau kedaluwarsa)
	case errors.Is(err, apperror.ErrGoogleInvalidToken):
		// kirim 401 tanpa detail alasan penolakan
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid google token")
	// Google tidak menjamin email itu milik pemegang akun
	case errors.Is(err, apperror.ErrGoogleEmailNotVerified):
		// kirim 403 GOOGLE_EMAIL_UNVERIFIED
		respondError(c, http.StatusForbidden, "GOOGLE_EMAIL_UNVERIFIED", "google account email is not verified")
	// kunci publik Google tidak bisa diambil
	case errors.Is(err, apperror.ErrGoogleUnavailable):
		// kirim 503 GOOGLE_UNAVAILABLE; klien boleh mencoba lagi nanti
		respondError(c, http.StatusServiceUnavailable, "GOOGLE_UNAVAILABLE", "google sign-in is temporarily unavailable")
	// akun sudah tertaut ke akun Google lain, atau tabrakan: 409
	case errors.Is(err, apperror.ErrDuplicate):
		// kirim 409 CONFLICT dengan pesan generik
		respondError(c, http.StatusConflict, "CONFLICT", "conflict")
	// galat lain: catat, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("google login failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// LinkGoogle: POST /api/v1/users/google/link. Menautkan akun Google setelah password akun terdaftar dibuktikan.
//   - 200 {data:{token}}; 400 INVALID_TOKEN: link token tidak sah, kedaluwarsa, atau sudah dibakar;
//   - 401 UNAUTHORIZED: password salah; 409 CONFLICT: akun sudah tertaut ke akun Google lain.
func (h *AuthHandler) LinkGoogle(c *gin.Context) {
	// respons memuat token: jangan di-cache
	c.Header("Cache-Control", "no-store")
	// baca body JSON; gagal berarti format request salah
	var req dto.LinkGoogleRequest
	// body JSON tidak sah ditolak sebelum penautan diproses
	if err := c.ShouldBindJSON(&req); err != nil {
		// balas 400 BAD_REQUEST dengan pesan generik
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// tautkan akun Google; galat diterjemahkan ke respons HTTP
	token, err := h.svc.LinkGoogle(c.Request.Context(), req.LinkToken, req.Password)
	// cek hasil penautan
	if err != nil {
		// serahkan ke penerjemah error, lalu hentikan
		h.respondLinkGoogleError(c, err)
		return
	}
	// berhasil: kirim token login
	respondData(c, http.StatusOK, dto.AuthResponse{Token: token})
}

// respondLinkGoogleError menerjemahkan error LinkGoogle menjadi respons HTTP.
func (h *AuthHandler) respondLinkGoogleError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error
	switch {
	// link token tidak sah, kedaluwarsa, atau sudah dibakar
	case errors.Is(err, apperror.ErrInvalidLinkToken):
		// kirim 400 INVALID_TOKEN
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "link token is invalid, expired or already used")
	// password salah: balasan sama dengan login yang gagal
	case errors.Is(err, apperror.ErrNotFound):
		// kirim 401 seperti login gagal
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
	// akun sudah tertaut ke akun Google lain: 409
	case errors.Is(err, apperror.ErrDuplicate):
		// kirim 409 CONFLICT dengan pesan generik
		respondError(c, http.StatusConflict, "CONFLICT", "conflict")
	// galat lain: catat, balas 500 tanpa detail
	default:
		// catat detail error hanya di log server
		h.log.Error("link google failed", "err", err)
		// klien hanya menerima 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
