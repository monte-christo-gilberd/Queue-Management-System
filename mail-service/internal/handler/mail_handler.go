package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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
	w http.ResponseWriter,
	r *http.Request,
) {
	// Batasi ukuran body agar pembacaan berhenti di 64 KB.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	// Siapkan struct permintaan dan decoder JSON yang menolak field yang tidak dikenal.
	var req dto.SendRequest
	dec := json.NewDecoder(r.Body)
	// Field tak dikenal ditolak agar salah ketik (mis. "tmplate") ketahuan, bukan diam-diam diabaikan.
	dec.DisallowUnknownFields()
	// Body tidak bisa didecode: bila karena melewati batas ukuran, balas 413; selain itu balas 400.
	if err := dec.Decode(&req); err != nil {
		// MaxBytesReader menghasilkan *http.MaxBytesError bila body melewati batas.
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			// Body terlalu besar: 413, bukan 400, agar pemanggil tahu penyebabnya.
			respondError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large")
			return
		}
		// JSON rusak atau field salah: 400 tanpa membocorkan detail decoder.
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// Kirim lewat service, lalu petakan galatnya ke kode HTTP yang sesuai.
	err := h.svc.Send(r.Context(), req.To, req.Template, req.Data)
	// Petakan galat service ke kode HTTP dengan errors.Is karena galat bisa dibungkus.
	switch {
	// Berhasil terkirim: balas 200.
	case err == nil:
		respondData(w, http.StatusOK, dto.SendResponse{Status: "sent"})
	// Alamat penerima kosong atau tidak sah: 400.
	case errors.Is(err, apperror.ErrInvalidRecipient):
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid recipient")
	// Nama templat tidak dikenal: 400.
	case errors.Is(err, apperror.ErrUnknownTemplate):
		respondError(w, http.StatusBadRequest, "UNKNOWN_TEMPLATE", "unknown email template")
	// Data tidak melengkapi templat: 400.
	case errors.Is(err, apperror.ErrBadData):
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "template data is incomplete")
	// Terlalu banyak pengiriman bersamaan: 503 agar pemanggil mencoba lagi nanti.
	case errors.Is(err, apperror.ErrBusy):
		respondError(w, http.StatusServiceUnavailable, "BUSY", "too many emails in flight, retry later")
	// Galat kirim: 502 tanpa membocorkan detail SMTP.
	case errors.Is(err, apperror.ErrSendFailed):
		// Pesan sengaja generik: detail SMTP hanya masuk log server.
		respondError(w, http.StatusBadGateway, "SMTP_FAILED", "email could not be sent")
	// Galat tidak dikenal (mis. render gagal) adalah kesalahan server: 500 tanpa detail.
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
