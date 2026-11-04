// Package mailclient: klien HTTP untuk API internal mail-service (POST /internal/v1/send).
// Pola sama dengan qrclient. Detail galat mail-service tidak diteruskan ke pengguna.
package mailclient

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

const defaultTimeout = 20 * time.Second

// ErrUnavailable: mail-service tidak terjangkau atau menolak pengiriman.
var ErrUnavailable = errors.New("mail service unavailable")

type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func New(baseURL, serviceToken string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

// Send meminta mail-service mengirim email dari templat `template` ke `to`.
// Galat tidak memuat URL, token, atau isi data (data bisa berisi tautan reset).
func (c *Client) Send(ctx context.Context, to, template string, data map[string]any) error {
	body, err := json.Marshal(map[string]any{"to": to, "template": template, "data": data})
	if err != nil {
		return ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/send", bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.serviceToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}
