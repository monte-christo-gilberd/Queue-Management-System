// Package paymentclient memanggil payment-service (checkout pembayaran BorderPay) lewat jaringan internal.
package paymentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"qms/internal/apperror"
)

// defaultTimeout: batas waktu satu checkout bila timeout yang diberikan tidak valid.
const defaultTimeout = 15 * time.Second

// CheckoutRequest: body POST /internal/v1/payments di payment-service.
type CheckoutRequest struct {
	OwnerID     int64  `json:"owner_id"`
	Amount      int64  `json:"amount"`
	Method      string `json:"method"`
	ReturnURL   string `json:"return_url,omitempty"`
	ReferenceID string `json:"reference_id"`
	PlanType    string `json:"plan_type"`
}

// CheckoutResponse: hasil checkout yang dikembalikan payment-service.
type CheckoutResponse struct {
	ReferenceID string `json:"reference_id"`
	Status      string `json:"status"`
	PayURL      string `json:"pay_url"`
	ExpiresAt   string `json:"expires_at"`
}

// Client: klien HTTP ke payment-service.
type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

// New membuat Client. Timeout yang tidak valid memakai nilai bawaan.
func New(baseURL, serviceToken string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

// Checkout membuat pembayaran di payment-service. Galat jaringan dan 5xx menjadi ErrPaymentUnavailable;
// penolakan 4xx menjadi ErrPaymentRejected (atau ErrInvalidPaymentMethod bila validasi gagal).
func (c *Client) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("paymentclient: encode: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/payments", bytes.NewReader(body))
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("paymentclient: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", c.serviceToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CheckoutResponse{}, apperror.ErrPaymentUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return CheckoutResponse{}, apperror.ErrPaymentUnavailable
	}

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
	case resp.StatusCode == http.StatusBadRequest:
		return CheckoutResponse{}, apperror.ErrInvalidPaymentMethod
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return CheckoutResponse{}, apperror.ErrPaymentUnavailable
	default:
		return CheckoutResponse{}, apperror.ErrPaymentRejected
	}

	var out CheckoutResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return CheckoutResponse{}, apperror.ErrPaymentUnavailable
	}
	// Tanpa pay_url customer tidak bisa membayar, jadi dianggap gagal dari sisi layanan pembayaran.
	if out.PayURL == "" || out.ReferenceID == "" {
		return CheckoutResponse{}, apperror.ErrPaymentUnavailable
	}
	return out, nil
}
