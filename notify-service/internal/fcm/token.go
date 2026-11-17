package fcm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"qms/notify-service/internal/apperror"
)

// scope: izin OAuth2 yang dibutuhkan untuk memanggil FCM HTTP v1. Access token hanya berlaku untuk scope ini.
const scope = "https://www.googleapis.com/auth/firebase.messaging"

// defaultTokenURI: alamat penukaran token OAuth2 Google, dipakai bila token_uri di file service account kosong.
const defaultTokenURI = "https://oauth2.googleapis.com/token"

// Credentials: isi file JSON service account Google yang dipakai FCM. Field lain di file (type, client_id, dll.)
// tidak dibutuhkan dan diabaikan.
type Credentials struct {
	// ProjectID: project Firebase (project_id). Dipakai bila FCM_PROJECT_ID kosong.
	ProjectID string `json:"project_id"`
	// ClientEmail: email service account (client_email), dipakai sebagai issuer (iss) JWT.
	ClientEmail string `json:"client_email"`
	// PrivateKey: kunci privat RSA berformat PEM (private_key) untuk menandatangani JWT. Rahasia.
	PrivateKey string `json:"private_key"`
	// TokenURI: alamat penukaran JWT ke access token (token_uri). Juga menjadi audience (aud) JWT.
	TokenURI string `json:"token_uri"`
}

// ParseCredentials membaca JSON service account dan memastikan field wajib terisi.
func ParseCredentials(data []byte) (Credentials, error) {
	// Ubah JSON ke struct; JSON rusak berarti berkas salah.
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return Credentials{}, fmt.Errorf("fcm credentials: %w", err)
	}
	// Tanpa email dan kunci privat, JWT tidak bisa dibuat sama sekali.
	if c.ClientEmail == "" || c.PrivateKey == "" {
		return Credentials{}, errors.New("fcm credentials: client_email and private_key are required")
	}
	// token_uri boleh kosong: pakai alamat bawaan Google.
	if c.TokenURI == "" {
		c.TokenURI = defaultTokenURI
	}
	return c, nil
}

// tokenSource menukar JWT bertanda tangan service account dengan access token OAuth2 (alur "JWT bearer grant"),
// lalu menyimpannya sampai 1 menit sebelum kedaluwarsa. Dengan begitu penukaran hanya terjadi kira-kira sekali
// per jam, bukan sekali per notifikasi. Aman dipakai bersamaan dari beberapa goroutine.
type tokenSource struct {
	// creds: kredensial service account.
	creds Credentials
	// key: kunci privat RSA hasil parse creds.PrivateKey.
	key *rsa.PrivateKey
	// client: klien HTTP untuk memanggil token_uri (sama dengan klien pengirim, batas waktu 8 detik).
	client *http.Client
	// now: sumber waktu; diganti di tes untuk menguji kedaluwarsa cache.
	now func() time.Time

	// mu menjaga token dan expires dari akses bersamaan.
	mu sync.Mutex
	// token: access token tersimpan; kosong berarti belum ada atau sudah dibuang (Invalidate).
	token string
	// expires: waktu kedaluwarsa token tersimpan.
	expires time.Time
}

// newTokenSource menyiapkan tokenSource; galat bila private_key bukan kunci RSA yang sah. Galat ini muncul saat
// start, sehingga kredensial rusak langsung terlihat di log, bukan saat notifikasi pertama dikirim.
func newTokenSource(creds Credentials, client *http.Client, now func() time.Time) (*tokenSource, error) {
	// Parse kunci privat sekali di awal.
	key, err := parseRSAKey(creds.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &tokenSource{creds: creds, key: key, client: client, now: now}, nil
}

// parseRSAKey membaca kunci privat PEM. Google memakai PKCS#8 ("BEGIN PRIVATE KEY"); PKCS#1
// ("BEGIN RSA PRIVATE KEY") juga diterima.
func parseRSAKey(pemKey string) (*rsa.PrivateKey, error) {
	// Ambil blok PEM pertama; tanpa blok berarti teksnya bukan PEM.
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("fcm credentials: private_key is not PEM")
	}
	// Coba PKCS#8 dulu (bentuk bawaan Google). Kunci selain RSA (mis. EC) tidak bisa dipakai RS256.
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("fcm credentials: private_key is not RSA")
	}
	// Bukan PKCS#8: coba PKCS#1.
	k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("fcm credentials: %w", err)
	}
	return k, nil
}

// Token mengembalikan access token yang masih berlaku, menukar JWT baru bila perlu.
func (s *tokenSource) Token(ctx context.Context) (string, error) {
	// Kunci dipegang selama penukaran, supaya beberapa pengiriman bersamaan tidak menukar token berkali-kali.
	s.mu.Lock()
	defer s.mu.Unlock()
	// Token masih berlaku lebih dari 1 menit: pakai ulang. Sisa 1 menit menjadi jeda aman terhadap selisih jam
	// dan lamanya request ke FCM.
	if s.token != "" && s.now().Add(time.Minute).Before(s.expires) {
		return s.token, nil
	}
	// Token belum ada atau hampir habis: tukar yang baru.
	tok, ttl, err := s.fetch(ctx)
	if err != nil {
		return "", err
	}
	// Simpan token beserta waktu kedaluwarsanya.
	s.token, s.expires = tok, s.now().Add(ttl)
	return s.token, nil
}

// Invalidate membuang token tersimpan, mis. setelah FCM menjawab 401 (token dicabut lebih awal). Pemanggilan
// Token berikutnya akan menukar token baru.
func (s *tokenSource) Invalidate() {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
}

// fetch menandatangani JWT lalu menukarnya di token_uri (OAuth2 JWT bearer grant, RFC 7523).
// Mengembalikan access token dan masa berlakunya.
func (s *tokenSource) fetch(ctx context.Context) (string, time.Duration, error) {
	// Buat JWT bertanda tangan kunci privat service account.
	assertion, err := s.signJWT()
	if err != nil {
		return "", 0, err
	}
	// Body form sesuai spesifikasi JWT bearer grant.
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.creds.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Kirim ke server token.
	resp, err := s.client.Do(req)
	if err != nil {
		// Konteks batal (service berhenti): kembalikan galat konteks, jangan dicoba ulang.
		if ctx.Err() != nil {
			return "", 0, ctx.Err()
		}
		// Galat jaringan: sementara, layak dicoba ulang oleh pusher.Retry.
		return "", 0, fmt.Errorf("%w: fcm oauth: %v", apperror.ErrTemporaryPush, err)
	}
	defer resp.Body.Close()
	// Baca body dengan batas 64 KiB supaya respons aneh tidak memakan memori.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	// Server token sibuk atau rusak sementara: layak dicoba ulang.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", 0, fmt.Errorf("%w: fcm oauth status %d", apperror.ErrTemporaryPush, resp.StatusCode)
	}
	// Status lain selain 200 (mis. 400 invalid_grant karena kunci dicabut): galat permanen, kredensial perlu
	// diperbaiki. Isi body disertakan karena berisi alasan dari Google (tidak memuat rahasia).
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("fcm oauth status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// Ambil access token dan masa berlakunya (detik).
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", 0, errors.New("fcm oauth: invalid token response")
	}
	// expires_in tidak ada: anggap 1 jam, sama dengan masa berlaku bawaan Google.
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	return out.AccessToken, time.Duration(out.ExpiresIn) * time.Second, nil
}

// signJWT membuat JWT RS256 untuk service account dengan scope FCM, berlaku 1 jam (batas maksimum Google).
func (s *tokenSource) signJWT() (string, error) {
	now := s.now().Unix()
	enc := base64.RawURLEncoding
	// Header: algoritma tanda tangan RS256.
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	// Klaim: siapa yang meminta (iss), izin yang diminta (scope), server tujuan (aud), dan masa berlaku.
	claims, _ := json.Marshal(map[string]any{
		"iss":   s.creds.ClientEmail,
		"scope": scope,
		"aud":   s.creds.TokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})
	// Bagian yang ditandatangani: header.klaim (masing-masing base64url tanpa padding).
	unsigned := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	// Tanda tangan RS256 = RSASSA-PKCS1-v1_5 atas SHA-256 dari bagian di atas.
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	// JWT lengkap: header.klaim.tanda_tangan.
	return unsigned + "." + enc.EncodeToString(sig), nil
}
