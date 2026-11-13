// Package checkout menyediakan endpoint internal untuk membuat pembayaran lewat BorderPay.
// Hanya backend utama yang boleh memanggil, dengan header X-Service-Token.
package checkout

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"qms/payment-service/internal/borderpay"
)

// maxBodyBytes: batas ukuran body request internal.
const maxBodyBytes = 16 << 10

// Creator membuat transaksi pembayaran (dipenuhi *borderpay.Client).
type Creator interface {
	CreatePayment(ctx context.Context, req borderpay.CreateRequest) (borderpay.CreateResponse, error)
}

// Request: body POST /internal/v1/payments dari backend utama.
type Request struct {
	Amount    int64  `json:"amount"`
	Method    string `json:"method"`
	BankCode  string `json:"bank_code"`
	ReturnURL string `json:"return_url"`
	// ReferenceID: ID pesanan dari backend. Bila kosong, layanan ini tidak membuatkan ID sendiri.
	ReferenceID string `json:"reference_id"`
}

// Response: data yang dikembalikan ke backend utama.
type Response struct {
	ReferenceID string `json:"reference_id"`
	Status      string `json:"status"`
	PayURL      string `json:"pay_url"`
	ExpiresAt   string `json:"expires_at"`
}

// Handler menangani POST /internal/v1/payments.
type Handler struct {
	serviceToken []byte
	creator      Creator
}

// NewHandler membuat Handler. Token layanan kosong berarti semua request ditolak.
func NewHandler(serviceToken string, creator Creator) *Handler {
	return &Handler{serviceToken: []byte(serviceToken), creator: creator}
}

// ServeHTTP memeriksa token layanan, memvalidasi request, lalu membuat pembayaran di BorderPay.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Hanya pemanggil dengan token layanan yang benar yang boleh membuat pembayaran.
	got := []byte(r.Header.Get("X-Service-Token"))
	if len(h.serviceToken) == 0 || subtle.ConstantTimeCompare(got, h.serviceToken) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req Request
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	out, err := h.creator.CreatePayment(r.Context(), borderpay.CreateRequest{
		Amount:      req.Amount,
		Method:      req.Method,
		BankCode:    strings.TrimSpace(req.BankCode),
		ReferenceID: strings.TrimSpace(req.ReferenceID),
		ReturnURL:   strings.TrimSpace(req.ReturnURL),
	})
	if err != nil {
		// Galat validasi menjadi 400; galat dari BorderPay menjadi 502 tanpa detail ke pemanggil.
		if errors.Is(err, borderpay.ErrInvalidRequest) {
			http.Error(w, "invalid payment request", http.StatusBadRequest)
			return
		}
		http.Error(w, "payment provider unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{
		ReferenceID: out.ReferenceID,
		Status:      out.Status,
		PayURL:      out.PayURL,
		ExpiresAt:   out.ExpiresAt,
	})
}
