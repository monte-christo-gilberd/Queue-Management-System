package router

import "github.com/gin-gonic/gin"

// registerAuthRoutes: registrasi dan login (selalu ada, tanpa login).
func registerAuthRoutes(
	rg *gin.RouterGroup,
	authH AuthHandler,
) {
	// buat akun baru
	rg.POST("/register", authH.Register)
	// masuk dengan email atau username
	rg.POST("/login", authH.Login)
}

// registerGoogleRoutes: login Google dan penautan akun (hanya bila Google dinyalakan).
func registerGoogleRoutes(
	rg *gin.RouterGroup,
	googleH GoogleHandler,
) {
	// tukar ID token Google dengan token login
	rg.POST("/google", googleH.GoogleLogin)
	// tautkan akun Google setelah password dibuktikan
	rg.POST("/google/link", googleH.LinkGoogle)
}

// registerEmailRoutes: reset password dan verifikasi email (hanya bila mail-service terpasang).
func registerEmailRoutes(
	rg *gin.RouterGroup,
	emailH EmailHandler,
) {
	// minta tautan reset password
	rg.POST("/forgot-password", emailH.ForgotPassword)
	// pasang password baru memakai token
	rg.POST("/reset-password", emailH.ResetPassword)
	// verifikasi email memakai token dari tautan
	rg.POST("/verify-email", emailH.VerifyEmail)
}

// registerMeRoutes: rute yang butuh login (hanya bila mail-service terpasang).
func registerMeRoutes(
	rg *gin.RouterGroup,
	emailH EmailHandler,
) {
	// kirim ulang tautan verifikasi untuk user yang login
	rg.POST("/verify-email/resend", emailH.ResendVerification)
}
