// Package borderpay berisi klien HTTP untuk REST API BorderPay (pembuatan pembayaran).
//
// Autentikasi: header `Authorization: Bearer <API key>` di setiap request. Key test berawalan `bp_test_`,
// key produksi berawalan `bp_live_`. Key hanya boleh ada di server, tidak pernah di frontend.
package borderpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Metode pembayaran yang didukung REST API BorderPay.
const (
	MethodQRIS    = "qris"
	MethodVA      = "va"
	MethodEwallet = "ewallet"
)

// ErrInvalidRequest: data pembuatan pembayaran tidak lengkap atau tidak valid (dicek sebelum request dikirim).
var ErrInvalidRequest = errors.New("borderpay: invalid payment request")

// ErrUpstream: BorderPay membalas dengan status gagal.
var ErrUpstream = errors.New("borderpay: upstream error")

// CreateRequest: body POST /api/v1/payments.
type CreateRequest struct {
	// Amount: nominal dalam rupiah (bilangan bulat, lebih dari nol).
	Amount int64 `json:"amount"`
	// Method: qris, va, atau ewallet.
	Method string `json:"method"`
	// BankCode: wajib untuk va (mis. BCA) dan ewallet (mis. DANA, OVO).
	BankCode string `json:"bank_code,omitempty"`
	// ReferenceID: ID pesanan dari sisi kita; harus unik per transaksi.
	ReferenceID string `json:"reference_id,omitempty"`
	// ReturnURL: tujuan setelah customer membayar. Harus https (kecuali key test boleh http://localhost).
	ReturnURL string `json:"return_url,omitempty"`
}

// CreateResponse: bagian response yang dipakai layanan ini.
type CreateResponse struct {
	ReferenceID string `json:"reference_id"`
	Status      string `json:"status"`
	Amount      int64  `json:"amount"`
	// PayURL: halaman pembayaran BorderPay yang dibuka customer.
	PayURL    string `json:"pay_url"`
	ExpiresAt string `json:"expires_at"`
}

// Client memanggil REST API BorderPay.
type Client struct {
	// baseURL: alamat dasar tanpa garis miring di akhir, mis. https://borderpay.id.
	baseURL string
	// apiKey: API key (bp_test_ atau bp_live_).
	apiKey string
	// http: klien HTTP dengan timeout.
	http *http.Client
}

// New membuat Client. baseURL dan apiKey wajib diisi.
func New(baseURL, apiKey string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("borderpay: base URL dan API key wajib diisi")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// Validate memeriksa aturan dasar sebelum request dikirim ke BorderPay.
func (r CreateRequest) Validate() error {
	// Nominal harus positif.
	if r.Amount <= 0 {
		return fmt.Errorf("%w: amount harus lebih dari 0", ErrInvalidRequest)
	}
	switch r.Method {
	case MethodQRIS:
		// QRIS tidak butuh kode bank.
	case MethodVA, MethodEwallet:
		// VA dan e-wallet wajib menyebut bank atau dompet digital.
		if strings.TrimSpace(r.BankCode) == "" {
			return fmt.Errorf("%w: bank_code wajib untuk metode %s", ErrInvalidRequest, r.Method)
		}
	default:
		return fmt.Errorf("%w: method tidak dikenal %q", ErrInvalidRequest, r.Method)
	}
	return nil
}

// CreatePayment membuat transaksi pembayaran dan mengembalikan pay_url untuk dibuka customer.
func (c *Client) CreatePayment(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	// Periksa dulu; request yang tidak valid tidak perlu dikirim.
	if err := req.Validate(); err != nil {
		return CreateResponse{}, err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return CreateResponse{}, fmt.Errorf("borderpay: encode request: %w", err)
	}
	// Susun request POST dengan header autentikasi dan JSON.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/payments", bytes.NewReader(body))
	if err != nil {
		return CreateResponse{}, fmt.Errorf("borderpay: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return CreateResponse{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	// Batasi ukuran respons supaya balasan tak wajar tidak membebani memori.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return CreateResponse{}, fmt.Errorf("%w: baca respons: %v", ErrUpstream, err)
	}
	// Status selain 201 (atau 200) dianggap gagal; isi respons tidak dibocorkan ke pemanggil.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return CreateResponse{}, fmt.Errorf("%w: status %d", ErrUpstream, resp.StatusCode)
	}
	var out CreateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return CreateResponse{}, fmt.Errorf("%w: respons tidak valid", ErrUpstream)
	}
	// Tanpa pay_url, customer tidak bisa membayar; anggap respons gagal.
	if out.PayURL == "" || out.ReferenceID == "" {
		return CreateResponse{}, fmt.Errorf("%w: pay_url atau reference_id kosong", ErrUpstream)
	}
	return out, nil
}
