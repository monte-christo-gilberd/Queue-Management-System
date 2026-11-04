// Package httpapi: API HTTP internal mail-service. Hanya dijangkau dari jaringan internal (tidak ada rute di
// nginx), dan setiap pemanggilan wajib membawa X-Service-Token, pola yang sama dengan qr-service.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"time"

	"qms/mail-service/internal/mailer"
)

const (
	maxBodyBytes = 64 << 10
	// maxInFlight membatasi pengiriman yang berjalan bersamaan; lebih dari itu dijawab 503 supaya pemanggil
	// yang membanjiri tidak menghabiskan koneksi SMTP.
	maxInFlight = 16
	sendTimeout = 45 * time.Second
)

// Sender: yang benar-benar mengirim email (mailer.Mailer; tes memakai tiruan).
type Sender interface {
	Send(ctx context.Context, msg mailer.Message) error
}

type API struct {
	sender Sender
	token  string
	sem    chan struct{}
	log    *slog.Logger
}

func New(sender Sender, token string, log *slog.Logger) *API {
	if log == nil {
		log = slog.Default()
	}
	return &API{sender: sender, token: token, sem: make(chan struct{}, maxInFlight), log: log}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("POST /internal/v1/send", a.auth(a.send))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	})
	return mux
}

func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Service-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) != 1 {
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		next(w, r)
	}
}

// SendRequest: {"to":"budi@contoh.id","template":"test","data":{...}}. Isi email ditentukan templat di
// mail-service; pemanggil hanya memberi nama templat dan datanya.
type SendRequest struct {
	To       string         `json:"to"`
	Template string         `json:"template"`
	Data     map[string]any `json:"data"`
}

// send mengirim satu email secara sinkron (dengan coba ulang di dalam Mailer) dan menjawab setelah selesai.
// Tidak ada antrian di memori: pemanggil yang butuh ketahanan terhadap mati mendadak memakai outbox (item 8).
func (a *API) send(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req SendRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if _, err := mail.ParseAddress(req.To); err != nil || req.To == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid recipient")
		return
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	rendered, err := mailer.Render(req.Template, req.Data)
	switch {
	case errors.Is(err, mailer.ErrUnknownTemplate):
		writeErr(w, http.StatusBadRequest, "UNKNOWN_TEMPLATE", "unknown email template")
		return
	case errors.Is(err, mailer.ErrBadData):
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "template data is incomplete")
		return
	case err != nil:
		a.log.Error("render", "template", req.Template, "err", err)
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	select {
	case a.sem <- struct{}{}:
		defer func() { <-a.sem }()
	default:
		writeErr(w, http.StatusServiceUnavailable, "BUSY", "too many emails in flight, retry later")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), sendTimeout)
	defer cancel()
	// Alamat penerima sengaja tidak dicatat di log (data pribadi); cukup nama templat dan hasilnya.
	if err := a.sender.Send(ctx, mailer.Message{To: req.To, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML}); err != nil {
		a.log.Error("send email", "template", req.Template, "permanent", errors.Is(err, mailer.ErrPermanent), "err", err)
		// Detail SMTP tidak dibocorkan ke pemanggil.
		writeErr(w, http.StatusBadGateway, "SMTP_FAILED", "email could not be sent")
		return
	}
	a.log.Info("email sent", "template", req.Template)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "sent"}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
