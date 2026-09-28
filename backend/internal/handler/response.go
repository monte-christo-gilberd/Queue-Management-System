package handler

import (
	"github.com/gin-gonic/gin"
)

func respondData(
	c *gin.Context,
	status int,
	data any,
) {
	c.JSON(
		status,
		gin.H{
			"data": data,
		},
	)
}

func respondError(
	c *gin.Context,
	status int,
	code, message string,
) {
	c.JSON(
		status,
		gin.H{
			"error": gin.H{
				"code":    code,
				"message": message,
			},
		},
	)
}
