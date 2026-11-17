// Package webhook menerima notifikasi status pembayaran dari BorderPay.
//
// Verifikasi (mode SIMPLE): header `x-borderpay-token` harus sama dengan verification token project
// (bpt_..., dari dashboard). Token salah atau kosong ditolak 401. Tanpa token yang dikonfigurasi,
// semua webhook ditolak (fail closed).
package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"qms/payment-service/internal/order"
)

// Nama event yang dikirim BorderPay di header x-borderpay-event. Konstanta ini tidak dipakai oleh kode: handler
// tidak membaca header maupun field event, hanya data.status (lihat ServeHTTP).
const (
	EventPaid    = "payment.paid"
	EventExpired = "payment.expired"
	EventFailed  = "payment.failed"
)

// maxBodyBytes: batas ukuran body webhook.
const maxBodyBytes = 64 << 10

// Payload: body webhook BorderPay.
type Payload struct {
	Event string `json:"event"`
	Mode  string `json:"mode"`
	Data  struct {
		Slug             string `json:"slug"`
		ReferenceID      string `json:"reference_id"`
		OrderID          string `json:"order_id"`
		Status           string `json:"status"`
		Amount           int64  `json:"amount"`
		Fee              int64  `json:"fee"`
		MerchantReceives int64  `json:"merchant_receives"`
		Method           string `json:"method"`
	} `json:"data"`
}

// PaidStore menyimpan hasil pembayaran lunas. Implementasi harus idempotent: pembayaran yang sama
// (referenceID) hanya diterapkan sekali. applied=false berarti sudah pernah diterapkan.
// Status expired yang lalu menjadi paid harus tetap dianggap paid (paid adalah status final).
type PaidStore interface {
	MarkPaid(ctx context.Context, referenceID string, amount int64) (applied bool, err error)
}

// Handler menangani POST webhook BorderPay.
type Handler struct {
	// token: verification token project. Kosong berarti semua request ditolak.
	token []byte
	store PaidStore
}

// NewHandler membuat Handler. Token boleh kosong, tetapi itu membuat semua webhook ditolak.
func NewHandler(verificationToken string, store PaidStore) *Handler {
	return &Handler{token: []byte(verificationToken), store: store}
}

// ServeHTTP memverifikasi token, membaca event, lalu menerapkan pembayaran lunas.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verifikasi token dengan perbandingan waktu-konstan. Token kosong di konfigurasi selalu ditolak.
	got := []byte(r.Header.Get("x-borderpay-token"))
	if len(h.token) == 0 || subtle.ConstantTimeCompare(got, h.token) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil || len(raw) > maxBodyBytes {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Yang menentukan adalah data.status = "paid" (bukan nama event payment.paid). Status lain, termasuk expired dan
	// failed, tidak mengubah data dan dibalas 200 agar BorderPay tidak mengulang.
	if payload.Data.Status != "paid" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if payload.Data.ReferenceID == "" {
		http.Error(w, "missing reference_id", http.StatusBadRequest)
		return
	}

	// Simpan dulu, baru balas 2xx. Bila penyimpanan gagal, balas 500 supaya BorderPay mengirim ulang.
	applied, err := h.store.MarkPaid(r.Context(), payload.Data.ReferenceID, payload.Data.Amount)
	// Pesanan tidak dikenal: tidak ada yang bisa diterapkan dan mengirim ulang tidak membantu, jadi dibalas 200.
	if errors.Is(err, order.ErrUnknown) {
		log.Printf("payment webhook: reference_id=%s tidak dikenal, diabaikan", payload.Data.ReferenceID)
		w.WriteHeader(http.StatusOK)
		return
	}
	// Nominal tidak sesuai: pembayaran tidak diterapkan. Dibalas 409 dan dicatat untuk ditelusuri.
	if errors.Is(err, order.ErrAmountMismatch) {
		log.Printf("payment webhook: nominal reference_id=%s tidak sesuai pesanan", payload.Data.ReferenceID)
		http.Error(w, "amount mismatch", http.StatusConflict)
		return
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("payment webhook: simpan pembayaran gagal: %v", err)
		http.Error(w, "temporary failure", http.StatusInternalServerError)
		return
	}
	if !applied {
		log.Printf("payment webhook: duplikat untuk reference_id=%s, diabaikan", payload.Data.ReferenceID)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
