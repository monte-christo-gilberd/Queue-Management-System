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

const defaultTimeout = 10 * time.Second

// JoinToken: hasil pembuatan token join oleh qr-service.
type JoinToken struct {
	Token     string `json:"token"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

// JoinTokenClaims: hasil verifikasi token join oleh qr-service.
type JoinTokenClaims struct {
	QueueID   int64  `json:"queue_id"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

// Client: HTTP client untuk endpoint internal qr-service.
type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func New(baseURL string, serviceToken string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
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
	body := map[string]interface{}{
		"queue_id": queueID,
		"type":     typ,
	}
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}

	resp, err := c.do(ctx, http.MethodPost, "/internal/v1/join-token", body)
	if err != nil {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return JoinToken{}, joinTokenIssueError(resp)
	}

	var out JoinToken
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	if out.Token == "" {
		return JoinToken{}, apperror.ErrQRUnavailable
	}
	return out, nil
}

// VerifyJoinToken memeriksa token join (tanda tangan + masa berlaku).
func (c *Client) VerifyJoinToken(
	ctx context.Context,
	token string,
) (JoinTokenClaims, error) {
	resp, err := c.do(ctx, http.MethodPost, "/internal/v1/join-token/verify", map[string]string{"token": token})
	if err != nil {
		return JoinTokenClaims{}, apperror.ErrQRUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return JoinTokenClaims{}, joinTokenVerifyError(resp)
	}

	var out JoinTokenClaims
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
		return JoinTokenClaims{}, apperror.ErrQRUnavailable
	}
	if out.QueueID <= 0 {
		return JoinTokenClaims{}, apperror.ErrQRInvalid
	}
	return out, nil
}

// do: kirim JSON request ke endpoint internal. Error jaringan -> ErrQRUnavailable.
func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body interface{},
) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, apperror.ErrQRUnavailable
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, apperror.ErrQRUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.serviceToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperror.ErrQRUnavailable
	}
	return resp, nil
}

func joinTokenIssueError(resp *http.Response) error {
	code, _ := qrErrorCode(resp)
	if resp.StatusCode == http.StatusBadRequest && code == "QR_TYPE_INVALID" {
		return apperror.ErrQRTypeInvalid
	}
	return apperror.ErrQRUnavailable
}

func joinTokenVerifyError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusGone:
		return apperror.ErrQRExpired
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound:
		return apperror.ErrQRInvalid
	default:
		return apperror.ErrQRUnavailable
	}
}

func qrErrorCode(resp *http.Response) (string, bool) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", false
	}
	var parsed struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false
	}
	return parsed.Error.Code, parsed.Error.Code != ""
}
