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

// defaultTimeout: batas waktu satu panggilan ke mail-service bila timeout yang diberikan tidak valid (nol atau negatif).
const defaultTimeout = 20 * time.Second

// ErrUnavailable: mail-service tidak terjangkau atau menolak pengiriman.
var ErrUnavailable = errors.New("mail service unavailable")

// Client adalah klien HTTP untuk mail-service.
type Client struct {
	// baseURL adalah alamat dasar mail-service tanpa "/" di akhir, mis. http://mail:8081.
	baseURL string
	// serviceToken adalah token rahasia yang dikirim di header X-Service-Token agar mail-service mengenali pemanggil.
	serviceToken string
	// httpClient adalah klien HTTP bawaan Go dengan batas waktu total per panggilan.
	httpClient *http.Client
}

// New membuat Client untuk baseURL dan serviceToken. Timeout nol atau negatif diganti defaultTimeout.
func New(baseURL, serviceToken string, timeout time.Duration) *Client {
	// timeout tidak valid: pakai nilai bawaan
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	// buat Client; "/" di akhir baseURL dibuang agar tidak ada "//" saat menggabungkan path
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

// Send meminta mail-service mengirim email dari templat `template` ke `to`.
// Galat tidak memuat URL, token, atau isi data (data bisa berisi tautan reset).
func (c *Client) Send(ctx context.Context, to, template string, data map[string]any) error {
	// ubah tujuan, nama templat, dan data menjadi JSON; gagal berarti data tidak bisa diserialisasi
	body, err := json.Marshal(map[string]any{"to": to, "template": template, "data": data})
	if err != nil {
		return ErrUnavailable
	}
	// buat request POST ke endpoint pengiriman, ikut context agar bisa dibatalkan pemanggil
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/send", bytes.NewReader(body))
	// gagal membuat request (misalnya URL tidak valid): anggap mail-service tidak tersedia
	if err != nil {
		return ErrUnavailable
	}
	// beri tahu mail-service bahwa isi request berformat JSON
	req.Header.Set("Content-Type", "application/json")
	// kirim token service agar mail-service menerima request ini
	req.Header.Set("X-Service-Token", c.serviceToken)
	// kirim request; error di sini berarti tidak terhubung, timeout, atau koneksi putus
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	// tutup body respons saat fungsi selesai agar koneksi bisa dipakai ulang
	defer resp.Body.Close()
	// buang isi respons (maksimal 64 KB) agar koneksi keep-alive bisa dipakai ulang; error diabaikan
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	// status selain 200 OK berarti pengiriman ditolak; status disertakan untuk log, bukan untuk pengguna
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	// status 200 OK: email berhasil diterima mail-service
	return nil
}
