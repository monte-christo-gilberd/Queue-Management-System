package handler

import (
	"net/http"
	"qms/qr-service/internal/dto"
	"qms/qr-service/internal/model"

	"github.com/gin-gonic/gin"
)

// RenderService adalah logika render QR menjadi PNG (dipenuhi oleh service).
type RenderService interface {
	// Render menggambar QR dari opsi dan mengembalikan bytes PNG.
	Render(
		opts model.RenderOptions,
	) ([]byte, error)
}

// RenderHandler menangani rute render QR umum.
type RenderHandler struct {
	// service berisi logika render.
	service RenderService
}

// NewRenderHandler membuat RenderHandler dengan service yang diberikan.
func NewRenderHandler(
	service RenderService,
) *RenderHandler {
	// isi handler dengan service
	return &RenderHandler{
		service: service,
	}
}

// Render merender QR apa pun untuk kebutuhan internal lain (mis. QR
// pembayaran). Tidak menerima data rahasia dari klien publik.
func (h *RenderHandler) Render(c *gin.Context) {
	// baca dan decode body JSON ke RenderRequest
	var req dto.RenderRequest
	// body tidak bisa dibaca atau bukan JSON yang sesuai: balas 400
	if err := c.ShouldBindJSON(&req); err != nil {
		// 400 dengan kode umum; detail kesalahan JSON tidak dibocorkan ke klien
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// render data menjadi PNG dengan opsi dari request
	png, err := h.service.Render(model.RenderOptions{
		Data:            req.Data,
		Size:            req.Size,
		RecoveryLevel:   req.Level,
		ForegroundColor: req.FG,
		BackgroundColor: req.BG,
	})
	// semua galat render (data, level, warna) dianggap permintaan salah; pesan dari service dikirim apa adanya
	if err != nil {
		// pesan galat dari service membantu klien memperbaiki data, level, atau warna
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// kirim PNG ke klien
	respondPNG(c, png)
}
