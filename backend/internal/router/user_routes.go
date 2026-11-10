package router

import "github.com/gin-gonic/gin"

// registerUserRoutes mendaftarkan rute publik user: register dan login (tanpa middleware login).
func registerUserRoutes(
	rg *gin.RouterGroup,
	userH UserHandler,
) {
	// daftarkan akun baru
	rg.POST("/register", userH.Register)
	// login dan terima token
	rg.POST("/login", userH.Login)
}
