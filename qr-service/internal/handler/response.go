package handler

import (
	"net/http"
	"qms/qr-service/internal/dto"

	"github.com/gin-gonic/gin"
)

// respondJSON menulis respons sukses 200 dengan body JSON polos (tanpa pembungkus "data").
func respondJSON(
	c *gin.Context,
	body interface{},
) {
	// kirim body apa adanya sebagai JSON
	c.JSON(http.StatusOK, body)
}

// respondPNG mengirim bytes PNG dengan header Content-Type yang benar dan nosniff.
func respondPNG(
	c *gin.Context,
	png []byte,
) {
	// jangan simpan gambar di cache
	c.Header("Cache-Control", "no-store")
	// tandai isi sebagai PNG dan larang browser menebak tipe lain
	c.Header("Content-Type", "image/png")
	c.Header("X-Content-Type-Options", "nosniff")
	// kirim bytes PNG dengan status 200
	c.Data(http.StatusOK, "image/png", png)
}

// respondError menulis respons error dengan status HTTP status dan isi JSON {"error": {"code", "message"}}.
// code adalah kode tetap yang dibaca klien, message adalah penjelasan singkat.
func respondError(
	c *gin.Context,
	status int,
	code, message string,
) {
	// bungkus kode dan pesan dalam field "error" lalu kirim sebagai JSON
	c.JSON(
		status,
		dto.ErrorResponse{
			Error: dto.ErrorBody{
				Code:    code,
				Message: message,
			},
		},
	)
}
