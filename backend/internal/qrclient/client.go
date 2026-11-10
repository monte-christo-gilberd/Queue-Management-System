package qrclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"qms/internal/apperror"
)

// defaultTimeout: batas waktu satu panggilan ke qr-service bila timeout yang diberikan tidak valid (nol atau negatif).
const defaultTimeout = 10 * time.Second

// JoinToken: hasil pembuatan token join oleh qr-service.
type JoinToken struct {
	// Token adalah string token yang akan ditampilkan sebagai QR code.
	Token string `json:"token"`
	// Type adalah jenis token join (misalnya dinamis atau statis), sesuai yang diminta saat membuat token.
	Type string `json:"type"`
	// ExpiresAt adalah waktu kedaluwarsa dalam detik Unix. Nil berarti token tidak punya batas waktu.
	ExpiresAt *int64 `json:"expires_at"`
}

// JoinTokenClaims: hasil verifikasi token join oleh qr-service.
type JoinTokenClaims struct {
	// QueueID adalah ID antrian yang dituju token ini.
	QueueID int64 `json:"queue_id"`
	// Type adalah jenis token join yang diverifikasi.
	Type string `json:"type"`
	// ExpiresAt adalah waktu kedaluwarsa dalam detik Unix. Nil berarti token tidak punya batas waktu.
	ExpiresAt *int64 `json:"expires_at"`
}

// Client: HTTP client untuk endpoint internal qr-service.
type Client struct {
	// baseURL adalah alamat dasar qr-service tanpa "/" di akhir.
	baseURL string
	// serviceToken adalah token rahasia yang dikirim di header X-Service-Token agar qr-service mengenali pemanggil.
	serviceToken string
	// httpClient adalah klien HTTP bawaan Go dengan batas waktu total per panggilan.
	httpClient *http.Client
}

// New membuat Client untuk baseURL dan serviceToken. Timeout nol atau negatif diganti defaultTimeout.
func New(baseURL string, serviceToken string, timeout time.Duration) *Client {
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

// IssueJoinToken membuat token join baru.
// ttlSeconds > 0 dipakai untuk token dinamis (maks. 600 dtk); 0 -> qr-service pakai default-nya.
func (c *Client) IssueJoinToken(
	ctx context.Context,
	queueID int64,
	typ string,
	ttlSeconds int,
) (JoinToken, error) {
	// isi body request dengan ID antrian dan jenis token
	body := map[string]interface{}{
		"queue_id": queueID,
		"type":     typ,
	}
	// ttl hanya dikirim bila positif; bila tidak, qr-service memakai masa berlaku bawaannya
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}

	// kirim POST ke endpoint pembuatan token; error jaringan berarti qr-service tidak tersedia
	resp, err := c.do(ctx, http.MethodPost, "/internal/v1/join-token", body)
	if err != nil {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	// tutup body respons saat fungsi selesai
	defer resp.Body.Close()
	// status selain 200 OK: terjemahkan kode error dari qr-service menjadi error aplikasi
	if resp.StatusCode != http.StatusOK {
		return JoinToken{}, joinTokenIssueError(resp)
	}

	// decode JSON respons; isi dibatasi 64 KB agar respons aneh tidak membebani memori
	var out JoinToken
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	// token kosong berarti respons tidak lengkap: anggap qr-service bermasalah
	if out.Token == "" {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	// token berhasil dibuat
	return out, nil
}

// VerifyJoinToken memeriksa token join (tanda tangan + masa berlaku).
func (c *Client) VerifyJoinToken(
	ctx context.Context,
	token string,
) (JoinTokenClaims, error) {
	// kirim token ke endpoint verifikasi; error jaringan berarti qr-service tidak tersedia
	resp, err := c.do(ctx, http.MethodPost, "/internal/v1/join-token/verify", map[string]string{"token": token})
	if err != nil {
		return JoinTokenClaims{}, apperror.ErrQRUnavailable
	}
	// tutup body respons saat fungsi selesai
	defer resp.Body.Close()
	// status selain 200 OK: terjemahkan menjadi error aplikasi (kedaluwarsa, tidak valid, atau tidak tersedia)
	if resp.StatusCode != http.StatusOK {
		return JoinTokenClaims{}, joinTokenVerifyError(resp)
	}

	// decode JSON klaim token; isi dibatasi 64 KB
	var out JoinTokenClaims
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
		return JoinTokenClaims{}, apperror.ErrQRUnavailable
	}
	// ID antrian harus positif; nilai nol atau negatif berarti klaim tidak valid
	if out.QueueID <= 0 {
		return JoinTokenClaims{}, apperror.ErrQRInvalid
	}
	// klaim valid
	return out, nil
}

// do: kirim JSON request ke endpoint internal. Error jaringan -> ErrQRUnavailable.
func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body interface{},
) (*http.Response, error) {
	// reader untuk isi request; tetap nil bila tidak ada body
	var reader io.Reader
	// hanya serialisasi JSON bila ada body
	if body != nil {
		// ubah body menjadi JSON; gagal berarti data tidak bisa dikirim
		b, err := json.Marshal(body)
		if err != nil {
			return nil, apperror.ErrQRUnavailable
		}
		reader = bytes.NewReader(b)
	}
	// buat request dengan method, URL lengkap (baseURL + path), dan isi, ikut context pemanggil
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, apperror.ErrQRUnavailable
	}
	// beri tahu qr-service bahwa isi request berformat JSON
	req.Header.Set("Content-Type", "application/json")
	// kirim token service agar qr-service menerima request ini
	req.Header.Set("X-Service-Token", c.serviceToken)
	// kirim request; error berarti tidak terhubung, timeout, atau koneksi putus
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperror.ErrQRUnavailable
	}
	// berhasil terhubung: kembalikan respons apa pun statusnya, pemeriksaan status dilakukan pemanggil
	return resp, nil
}

// joinTokenIssueError menerjemahkan respons gagal dari pembuatan token menjadi error aplikasi.
func joinTokenIssueError(resp *http.Response) error {
	// ambil kode error dari body JSON; kode kosong bila body tidak bisa dibaca
	code, _ := qrErrorCode(resp)
	// status 400 dengan kode QR_TYPE_INVALID berarti jenis token tidak dikenal
	if resp.StatusCode == http.StatusBadRequest && code == "QR_TYPE_INVALID" {
		return apperror.ErrQRTypeInvalid
	}
	// kondisi lain dianggap qr-service sedang bermasalah
	return apperror.ErrQRUnavailable
}

// joinTokenVerifyError menerjemahkan status HTTP dari verifikasi token menjadi error aplikasi.
func joinTokenVerifyError(resp *http.Response) error {
	// pilih error berdasarkan status HTTP
	switch resp.StatusCode {
	// 410 Gone: token sudah kedaluwarsa
	case http.StatusGone:
		return apperror.ErrQRExpired
	// 400, 401, atau 404: token tidak valid (format salah, tanda tangan salah, atau tidak ditemukan)
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound:
		return apperror.ErrQRInvalid
	// status lain: qr-service bermasalah
	default:
		return apperror.ErrQRUnavailable
	}
}

// qrErrorCode membaca kode error dari body JSON respons (format {"error": {"code": "..."}}).
// Mengembalikan kode dan true bila kode ditemukan; selain itu "" dan false.
func qrErrorCode(resp *http.Response) (string, bool) {
	// baca maksimal 4 KB dari body; gagal membaca berarti tidak ada kode yang bisa diambil
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", false
	}
	// struktur JSON yang diharapkan: objek "error" yang berisi "code"
	var parsed struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	// body bukan JSON yang sesuai struktur di atas: tidak ada kode
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false
	}
	// kode ditemukan bila tidak kosong
	return parsed.Error.Code, parsed.Error.Code != ""
}
