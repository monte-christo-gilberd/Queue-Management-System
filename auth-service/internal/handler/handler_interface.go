package handler

import "github.com/gin-gonic/gin"

// AuthHandlerInterface adalah kontrak handler akun yang dipakai oleh router.
type AuthHandlerInterface interface {
	// Register membuat akun baru dan mengembalikan token login.
	Register(c *gin.Context)
	// Login memeriksa kredensial dan mengembalikan token login.
	Login(c *gin.Context)
	// ForgotPassword meminta tautan reset password lewat email.
	ForgotPassword(c *gin.Context)
	// ResetPassword memasang password baru memakai token dari email.
	ResetPassword(c *gin.Context)
	// VerifyEmail menandai email terverifikasi memakai token dari email.
	VerifyEmail(c *gin.Context)
	// ResendVerification mengirim ulang tautan verifikasi untuk user yang login.
	ResendVerification(c *gin.Context)
	// GoogleLogin menukar ID token Google dengan token login.
	GoogleLogin(c *gin.Context)
	// LinkGoogle menautkan akun Google setelah password dibuktikan.
	LinkGoogle(c *gin.Context)
}
