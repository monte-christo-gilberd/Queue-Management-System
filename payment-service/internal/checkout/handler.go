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
	"qms/payment-service/internal/order"
)

// maxBodyBytes: batas ukuran body request internal.
const maxBodyBytes = 16 << 10

// Creator membuat transaksi pembayaran (dipenuhi *borderpay.Client).
type Creator interface {
	CreatePayment(ctx context.Context, req borderpay.CreateRequest) (borderpay.CreateResponse, error)
}

// Recorder menyimpan pesanan checkout (dipenuhi store.Postgres).
type Recorder interface {
	CreatePending(ctx context.Context, o order.Pending) error
	SetPayURL(ctx context.Context, referenceID, payURL string) error
}

// Request: body POST /internal/v1/payments dari backend utama.
type Request struct {
	// OwnerID: owner yang membayar langganan.
	OwnerID   int64  `json:"owner_id"`
	Amount    int64  `json:"amount"`
	Method    string `json:"method"`
	BankCode  string `json:"bank_code"`
	ReturnURL string `json:"return_url"`
	// ReferenceID: ID pesanan dari backend. Wajib dan unik; dipakai sebagai kunci di webhook.
	ReferenceID string `json:"reference_id"`
	// PlanType: paket yang dibeli. Kosong berarti order.PlanMonthly.
	PlanType string `json:"plan_type"`
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
	recorder     Recorder
}

// NewHandler membuat Handler. Token layanan kosong berarti semua request ditolak.
func NewHandler(serviceToken string, creator Creator, recorder Recorder) *Handler {
	return &Handler{serviceToken: []byte(serviceToken), creator: creator, recorder: recorder}
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

	req.ReferenceID = strings.TrimSpace(req.ReferenceID)
	if req.ReferenceID == "" || req.OwnerID <= 0 {
		http.Error(w, "reference_id and owner_id are required", http.StatusBadRequest)
		return
	}
	if req.PlanType == "" {
		req.PlanType = order.PlanMonthly
	}

	// Catat pesanan dulu sebagai pending, supaya webhook yang datang cepat tetap menemukannya.
	err := h.recorder.CreatePending(r.Context(), order.Pending{
		ReferenceID: req.ReferenceID,
		OwnerID:     req.OwnerID,
		Amount:      req.Amount,
		PlanType:    req.PlanType,
	})
	if errors.Is(err, order.ErrDuplicate) {
		http.Error(w, "reference_id already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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
	// Simpan pay_url ke pesanan. Tanpa pay_url tersimpan, webhook tetap bisa mencocokkan pesanan, tetapi
	// pemanggil sudah menerima pay_url yang tidak tercatat, jadi galat ini dibalas 500.
	if err := h.recorder.SetPayURL(r.Context(), req.ReferenceID, out.PayURL); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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
