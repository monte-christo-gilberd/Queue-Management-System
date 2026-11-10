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

// defaultTimeout: batas satu panggilan ke mail-service bila timeout yang diberikan tidak valid.
const defaultTimeout = 20 * time.Second

// ErrUnavailable: mail-service tidak terjangkau atau menolak pengiriman.
var ErrUnavailable = errors.New("mail service unavailable")

// Client adalah klien HTTP untuk mail-service.
type Client struct {
	// baseURL: alamat dasar mail-service tanpa "/" di akhir, mis. http://mail-service:8092.
	baseURL string
	// serviceToken: dikirim di header X-Service-Token agar mail-service mengenali pemanggil.
	serviceToken string
	// httpClient: klien HTTP dengan batas waktu total per panggilan.
	httpClient *http.Client
}

// New membuat Client. Timeout nol atau negatif diganti defaultTimeout.
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
	// ubah tujuan, nama templat, dan data menjadi JSON
	body, err := json.Marshal(map[string]any{"to": to, "template": template, "data": data})
	if err != nil {
		return ErrUnavailable
	}
	// buat request POST ke endpoint pengiriman; ikut context agar bisa dibatalkan pemanggil
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/send", bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	// isi request berformat JSON, dan token service agar mail-service menerima pemanggil ini
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.serviceToken)
	// kirim; galat berarti tidak terhubung, timeout, atau koneksi putus
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	// tutup body agar koneksi keep-alive bisa dipakai ulang
	defer resp.Body.Close()
	// buang isi respons (maksimal 64 KB); galat diabaikan
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	// selain 200 OK berarti ditolak; status ikut galat untuk log, bukan untuk pengguna
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}
