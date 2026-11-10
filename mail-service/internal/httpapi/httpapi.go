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
	// maxBodyBytes: batas ukuran body permintaan kirim (64 KB).
	maxBodyBytes = 64 << 10
	// maxInFlight membatasi pengiriman yang berjalan bersamaan; lebih dari itu dijawab 503 supaya pemanggil
	// yang membanjiri tidak menghabiskan koneksi SMTP.
	maxInFlight = 16
	// sendTimeout: batas waktu satu pengiriman sinkron, termasuk coba ulang (45 detik).
	sendTimeout = 45 * time.Second
)

// Sender: yang benar-benar mengirim email (mailer.Mailer; tes memakai tiruan).
type Sender interface {
	// Send mengirim satu pesan email.
	Send(ctx context.Context, msg mailer.Message) error
}

// API: handler HTTP untuk kirim email dan health check.
type API struct {
	// sender: pengirim email.
	sender Sender
	// token: nilai X-Service-Token yang harus dibawa pemanggil.
	token string
	// sem: semafor pembatas pengiriman bersamaan, berkapasitas maxInFlight.
	sem chan struct{}
	// log: pencatat peristiwa kirim dan galat.
	log *slog.Logger
}

// New membuat API dengan pengirim, token layanan, dan logger. Logger nil memakai slog default.
func New(sender Sender, token string, log *slog.Logger) *API {
	// Pakai logger default bila tidak diisi.
	if log == nil {
		log = slog.Default()
	}
	// Susun API dengan semafor berkapasitas maxInFlight.
	return &API{sender: sender, token: token, sem: make(chan struct{}, maxInFlight), log: log}
}

// Handler membangun router HTTP: health check, endpoint kirim (wajib token), dan 404 untuk rute lain.
func (a *API) Handler() http.Handler {
	// Router standar Go dengan pola metode dan path.
	mux := http.NewServeMux()
	// Health check: selalu 200 selama proses hidup.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	// Endpoint kirim: dibungkus auth sehingga token diperiksa dulu.
	mux.HandleFunc("POST /internal/v1/send", a.auth(a.send))
	// Rute lain: balas 404 dengan format error yang seragam.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	})
	return mux
}

// auth membungkus handler dan menolak request yang X-Service-Token-nya tidak cocok (perbandingan dalam waktu tetap).
func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Ambil token dari header request.
		got := r.Header.Get("X-Service-Token")
		// Token tidak cocok: tolak dengan 401 dan jangan jalankan handler.
		if subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) != 1 {
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		// Token cocok: jalankan handler berikutnya.
		next(w, r)
	}
}

// SendRequest: {"to":"budi@contoh.id","template":"test","data":{...}}. Isi email ditentukan templat di
// mail-service; pemanggil hanya memberi nama templat dan datanya.
type SendRequest struct {
	// To: alamat email penerima.
	To string `json:"to"`
	// Template: nama templat email, mis. "test" atau "password-reset".
	Template string `json:"template"`
	// Data: nilai yang mengisi templat. Kunci yang hilang membuat permintaan ditolak.
	Data map[string]any `json:"data"`
}

// send mengirim satu email secara sinkron (dengan coba ulang di dalam Mailer) dan menjawab setelah selesai.
// Tidak ada antrian di memori: pemanggil yang butuh ketahanan terhadap mati mendadak memakai outbox (item 8).
func (a *API) send(w http.ResponseWriter, r *http.Request) {
	// Batasi ukuran body agar pembacaan berhenti di 64 KB.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	// Siapkan struct permintaan dan decoder JSON yang menolak field yang tidak dikenal.
	var req SendRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	// Body tidak bisa didecode: bila karena melewati batas ukuran, balas 413; selain itu balas 400.
	if err := dec.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// Alamat penerima kosong atau tidak bisa diurai sebagai email: tolak.
	if _, err := mail.ParseAddress(req.To); err != nil || req.To == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid recipient")
		return
	}
	// Data tidak dikirim: pakai map kosong agar templat tetap bisa diisi.
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	// Isi templat dengan data. Galat dipetakan ke kode HTTP yang sesuai.
	rendered, err := mailer.Render(req.Template, req.Data)
	switch {
	// Nama templat tidak dikenal: 400.
	case errors.Is(err, mailer.ErrUnknownTemplate):
		writeErr(w, http.StatusBadRequest, "UNKNOWN_TEMPLATE", "unknown email template")
		return
	// Data tidak melengkapi templat: 400.
	case errors.Is(err, mailer.ErrBadData):
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "template data is incomplete")
		return
	// Galat render lain adalah kesalahan server: catat dan balas 500 tanpa detail.
	case err != nil:
		a.log.Error("render", "template", req.Template, "err", err)
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// Ambil satu slot dari semafor. Bila penuh, langsung balas 503 agar pemanggil mencoba lagi nanti.
	select {
	case a.sem <- struct{}{}:
		// Lepaskan slot setelah pengiriman selesai.
		defer func() { <-a.sem }()
	default:
		writeErr(w, http.StatusServiceUnavailable, "BUSY", "too many emails in flight, retry later")
		return
	}

	// Beri pengiriman batas waktu sendTimeout, mengikuti konteks permintaan.
	ctx, cancel := context.WithTimeout(r.Context(), sendTimeout)
	defer cancel()
	// Alamat penerima sengaja tidak dicatat di log (data pribadi); cukup nama templat dan hasilnya.
	if err := a.sender.Send(ctx, mailer.Message{To: req.To, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML}); err != nil {
		// Galat kirim: catat apakah permanen, lalu balas 502 tanpa membocorkan detail SMTP.
		a.log.Error("send email", "template", req.Template, "permanent", errors.Is(err, mailer.ErrPermanent), "err", err)
		// Detail SMTP tidak dibocorkan ke pemanggil.
		writeErr(w, http.StatusBadGateway, "SMTP_FAILED", "email could not be sent")
		return
	}
	// Berhasil terkirim: catat nama templat (bukan alamat) dan balas 200.
	a.log.Info("email sent", "template", req.Template)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "sent"}})
}

// writeJSON menulis respons JSON dengan status tertentu dan melarang cache.
func writeJSON(w http.ResponseWriter, status int, v any) {
	// Tandai respons sebagai JSON dan larang cache.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Tulis status, lalu encode nilai ke body. Galat encode diabaikan karena header sudah terkirim.
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr menulis respons error dengan kode dan pesan dalam format yang seragam.
func writeErr(w http.ResponseWriter, status int, code, msg string) {
	// Bungkus kode dan pesan ke bentuk error standar lalu tulis sebagai JSON.
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
