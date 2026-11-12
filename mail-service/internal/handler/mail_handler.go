package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/dto"
)

// maxBodyBytes: batas ukuran body permintaan kirim (64 KB).
const maxBodyBytes = 64 << 10

// MailService adalah logika kirim email langsung (dipenuhi oleh service.MailService).
type MailService interface {
	// Send memeriksa penerima, mengisi templat, lalu mengirim email secara sinkron.
	Send(
		ctx context.Context,
		to string,
		template string,
		data map[string]any,
	) error
}

// MailHandler: handler HTTP untuk kirim email. Dibuat lewat NewMailHandler.
type MailHandler struct {
	// svc: logika kirim email.
	svc MailService
}

// NewMailHandler membuat MailHandler dengan service kirim email.
func NewMailHandler(
	svc MailService,
) *MailHandler {
	// simpan service untuk dipanggil oleh Send
	return &MailHandler{svc: svc}
}

// Send menangani POST /internal/v1/send: membaca body, memanggil service, lalu menjawab setelah email terkirim.
func (h *MailHandler) Send(
	c *gin.Context,
) {
	// Batasi ukuran body agar pembacaan berhenti di 64 KB.
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		maxBodyBytes,
	)
	// Siapkan struct permintaan dan decoder JSON yang menolak field yang tidak dikenal.
	var req dto.SendRequest
	dec := json.NewDecoder(c.Request.Body)
	// Field tak dikenal ditolak agar salah ketik (mis. "tmplate") ketahuan, bukan diam-diam diabaikan.
	dec.DisallowUnknownFields()
	// Body tidak bisa didecode: bila karena melewati batas ukuran, balas 413; selain itu balas 400.
	if err := dec.Decode(&req); err != nil {
		// MaxBytesReader menghasilkan *http.MaxBytesError bila body melewati batas.
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			// Body terlalu besar: 413, bukan 400, agar pemanggil tahu penyebabnya.
			respondError(
				c, http.StatusRequestEntityTooLarge,
				"PAYLOAD_TOO_LARGE",
				"request body too large",
			)
			return
		}
		// JSON rusak atau field salah: 400 tanpa membocorkan detail decoder.
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}
	// Kirim lewat service, lalu petakan errornya ke kode HTTP yang sesuai.
	err := h.svc.Send(
		c.Request.Context(),
		req.To,
		req.Template,
		req.Data,
	)
	// Petakan error service ke kode HTTP dengan errors.Is karena error bisa dibungkus.
	switch {
	// Berhasil terkirim: balas 200.
	case err == nil:
		respondData(
			c,
			http.StatusOK,
			dto.SendResponse{
				Status: "sent",
			},
		)
	// Alamat penerima kosong atau tidak sah: 400.
	case errors.Is(err, apperror.ErrInvalidRecipient):
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid recipient",
		)
	// Nama templat tidak dikenal: 400.
	case errors.Is(err, apperror.ErrUnknownTemplate):
		respondError(
			c,
			http.StatusBadRequest,
			"UNKNOWN_TEMPLATE",
			"unknown email template",
		)
	// Data tidak melengkapi templat: 400.
	case errors.Is(err, apperror.ErrBadData):
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"template data is incomplete",
		)
	// Terlalu banyak pengiriman bersamaan: 503 agar pemanggil mencoba lagi nanti.
	case errors.Is(err, apperror.ErrBusy):
		respondError(
			c,
			http.StatusServiceUnavailable,
			"BUSY",
			"too many emails in flight, retry later",
		)
	// error kirim: 502 tanpa membocorkan detail SMTP.
	case errors.Is(err, apperror.ErrSendFailed):
		// Pesan sengaja generik: detail SMTP hanya masuk log server.
		respondError(
			c,
			http.StatusBadGateway,
			"SMTP_FAILED",
			"email could not be sent",
		)
	// error tidak dikenal (mis. render gagal) adalah kesalahan server: 500 tanpa detail.
	default:
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
	}
}
