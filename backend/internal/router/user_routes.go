package router

import "github.com/gin-gonic/gin"

func registerUserRoutes(
	rg *gin.RouterGroup,
	userH UserHandler,
) {
	rg.POST("/register", userH.Register)
	rg.POST("/login", userH.Login)
}
