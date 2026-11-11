package middleware

import (
	"qms/qr-service/internal/dto"

	"github.com/gin-gonic/gin"
)

// abortWithError menghentikan request dan mengirim respons error dalam bentuk dto.ErrorResponse.
// Setelah dipanggil, handler berikutnya dalam rantai tidak dijalankan.
func abortWithError(
	c *gin.Context,
	status int,
	code, message string,
) {
	// tulis status dan body JSON error, lalu hentikan rantai handler
	c.AbortWithStatusJSON(
		status,
		dto.ErrorResponse{
			Error: dto.ErrorBody{
				Code:    code,
				Message: message,
			},
		},
	)
}
