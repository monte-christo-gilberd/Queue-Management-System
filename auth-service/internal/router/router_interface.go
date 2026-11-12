package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// AuthHandler adalah handler untuk rute akun yang selalu ada: registrasi dan login.
type AuthHandler interface {
	// Register membuat akun baru dan mengembalikan token login.
	Register(c *gin.Context)
	// Login memeriksa kredensial dan mengembalikan token login.
	Login(c *gin.Context)
}

// EmailHandler adalah handler untuk rute yang butuh mail-service: reset password dan verifikasi email.
type EmailHandler interface {
	// ForgotPassword meminta tautan reset password lewat email.
	ForgotPassword(c *gin.Context)
	// ResetPassword memasang password baru memakai token dari email.
	ResetPassword(c *gin.Context)
	// VerifyEmail menandai email terverifikasi memakai token dari email.
	VerifyEmail(c *gin.Context)
	// ResendVerification mengirim ulang tautan verifikasi untuk user yang login.
	ResendVerification(c *gin.Context)
}

// GoogleHandler adalah handler untuk login Google.
type GoogleHandler interface {
	// GoogleLogin menukar ID token Google dengan token login.
	GoogleLogin(c *gin.Context)
	// LinkGoogle menautkan akun Google setelah password dibuktikan.
	LinkGoogle(c *gin.Context)
}

// AppRouter menyimpan semua handler dan middleware yang akan dipasang ke rute. Dibuat lewat NewAppRouter.
// Handler opsional bernilai nil bila tidak dipasang, dan rutenya juga tidak didaftarkan (404).
type AppRouter struct {
	// authHandler menangani registrasi dan login.
	authHandler AuthHandler
	// emailHandler menangani reset password dan verifikasi email (opsional; butuh mail-service).
	emailHandler EmailHandler
	// googleHandler menangani login Google (opsional; butuh GOOGLE_CLIENT_IDS).
	googleHandler GoogleHandler
	// authMiddleware memeriksa login sebelum rute yang butuh login diproses.
	authMiddleware gin.HandlerFunc
	// trustedProxies adalah daftar CIDR proxy yang boleh menentukan IP klien lewat X-Forwarded-For.
	trustedProxies []string
	// logger mencatat log akses dan panic (opsional; nil berarti slog.Default()).
	logger *slog.Logger
	// metricsMiddleware mencatat metrik HTTP (opsional; nil berarti tanpa metrik).
	metricsMiddleware gin.HandlerFunc
}
