package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/auth"
)

const (
	// DefaultCertsURL: kunci publik (JWKS) penanda ID token Google.
	DefaultCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

	// leeway: toleransi selisih jam server terhadap Google untuk klaim iat dan exp.
	leeway = 60 * time.Second
	// defaultKeyTTL dipakai bila respons Google tidak memuat Cache-Control max-age.
	defaultKeyTTL = time.Hour
	// minRefreshInterval: batas paling cepat mengambil ulang kunci karena `kid` tidak dikenal, supaya token
	// dengan kid acak tidak membuat server membanjiri Google.
	minRefreshInterval = time.Minute
	// maxKeysBody membatasi isi respons JWKS yang dibaca.
	maxKeysBody = 1 << 20
)

// issuers: nilai klaim `iss` yang sah untuk ID token Google.
var issuers = map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true}

// Verifier memeriksa ID token Google. Aman dipakai dari banyak goroutine.
type Verifier struct {
	// certsURL: alamat JWKS Google (bisa diganti di tes).
	certsURL string
	// audiences: client ID yang sah (web, Android, iOS). `aud` token harus salah satunya.
	audiences map[string]bool
	// client: klien HTTP dengan batas waktu untuk mengambil JWKS.
	client *http.Client
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time

	mu sync.Mutex
	// keys: kunci publik per `kid`.
	keys map[string]*rsa.PublicKey
	// expires: kapan keys harus diambil ulang.
	expires time.Time
	// lastFetch: kapan terakhir mengambil JWKS (untuk throttle kid tak dikenal).
	lastFetch time.Time
}

// NewVerifier membuat Verifier untuk daftar client ID. certsURL kosong = DefaultCertsURL.
func NewVerifier(certsURL string, clientIDs []string) *Verifier {
	if certsURL == "" {
		certsURL = DefaultCertsURL
	}
	aud := make(map[string]bool, len(clientIDs))
	for _, id := range clientIDs {
		if id = strings.TrimSpace(id); id != "" {
			aud[id] = true
		}
	}
	return &Verifier{
		certsURL:  certsURL,
		audiences: aud,
		client:    &http.Client{Timeout: 5 * time.Second},
		now:       time.Now,
		keys:      map[string]*rsa.PublicKey{},
	}
}

// WithClock mengganti sumber waktu. Hanya untuk tes (kedaluwarsa dan rotasi kunci tanpa menunggu).
func (v *Verifier) WithClock(now func() time.Time) *Verifier {
	v.now = now
	return v
}

// Verify memeriksa ID token dan mengembalikan identitasnya. Galat:
//   - apperror.ErrGoogleInvalidToken: tanda tangan, issuer, audience, atau masa berlaku tidak sah;
//   - apperror.ErrGoogleEmailNotVerified: Google tidak menjamin email itu milik pemegang akun;
//   - apperror.ErrGoogleUnavailable: kunci Google tidak bisa diambil.
func (v *Verifier) Verify(ctx context.Context, idToken string) (auth.GoogleIdentity, error) {
	// Tanpa daftar client ID, tidak ada audience yang sah: semua token ditolak.
	if len(v.audiences) == 0 {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}

	// Klaim exp/iat diperiksa sendiri di bawah dengan toleransi jam (pustaka tidak punya leeway).
	parser := &jwt.Parser{ValidMethods: []string{"RS256"}, SkipClaimsValidation: true}
	claims := jwt.MapClaims{}
	_, err := parser.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		return v.key(ctx, kid)
	})
	if err != nil {
		// Kunci tidak bisa diambil: bukan salah token, jadi dibedakan dari token tidak sah.
		var ve *jwt.ValidationError
		if errors.As(err, &ve) && errors.Is(ve.Inner, apperror.ErrGoogleUnavailable) {
			return auth.GoogleIdentity{}, apperror.ErrGoogleUnavailable
		}
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}

	now := v.now()
	// exp wajib dan harus di masa depan (dengan toleransi).
	exp, ok := numeric(claims["exp"])
	if !ok || now.After(time.Unix(exp, 0).Add(leeway)) {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// iat tidak boleh di masa depan (dengan toleransi).
	if iat, ok := numeric(claims["iat"]); ok && time.Unix(iat, 0).After(now.Add(leeway)) {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Issuer harus Google.
	if iss, _ := claims["iss"].(string); !issuers[iss] {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Audience harus salah satu client ID kita. Token untuk aplikasi lain ditolak.
	if !v.audienceOK(claims["aud"]) {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if sub == "" || email == "" {
		return auth.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	// Email yang tidak diverifikasi Google tidak boleh dipakai untuk membuat atau mencocokkan akun.
	if !truthy(claims["email_verified"]) {
		return auth.GoogleIdentity{}, apperror.ErrGoogleEmailNotVerified
	}
	name, _ := claims["name"].(string)
	return auth.GoogleIdentity{Subject: sub, Email: email, Name: name}, nil
}

// audienceOK: `aud` berupa string, atau daftar string; cukup satu yang cocok dengan client ID kita.
func (v *Verifier) audienceOK(aud interface{}) bool {
	switch a := aud.(type) {
	case string:
		return v.audiences[a]
	case []interface{}:
		for _, item := range a {
			if s, ok := item.(string); ok && v.audiences[s] {
				return true
			}
		}
	}
	return false
}

// numeric membaca klaim waktu (JSON mengubah angka menjadi float64).
func numeric(v interface{}) (int64, bool) {
	f, ok := v.(float64)
	return int64(f), ok
}

// truthy membaca email_verified: Google mengirim boolean, tetapi beberapa token memakai string "true".
func truthy(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}

// key mengembalikan kunci publik untuk kid, mengambil ulang JWKS bila perlu.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now()
	if k, ok := v.keys[kid]; ok && now.Before(v.expires) {
		return k, nil
	}
	// Ambil ulang bila cache kedaluwarsa, atau kid tak dikenal dan sudah lewat batas minimal sejak pengambilan terakhir
	// (Google merotasi kunci; kid baru harus bisa dikenali tanpa menunggu TTL).
	expired := !now.Before(v.expires)
	if expired || now.Sub(v.lastFetch) >= minRefreshInterval {
		if err := v.fetch(ctx); err != nil {
			// Cache lama yang kedaluwarsa tidak dipakai: lebih aman gagal daripada menerima kunci yang mungkin dicabut.
			return nil, fmt.Errorf("%w: %v", apperror.ErrGoogleUnavailable, err)
		}
	}
	k, ok := v.keys[kid]
	if !ok {
		return nil, errors.New("unknown kid")
	}
	return k, nil
}

// jwks: bentuk respons JWKS Google.
type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// fetch mengambil JWKS dan mengganti cache. Dipanggil dengan v.mu terkunci.
func (v *Verifier) fetch(ctx context.Context) error {
	v.lastFetch = v.now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	var set jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxKeysBody)).Decode(&set); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("no usable keys")
	}
	v.keys = keys
	v.expires = v.now().Add(maxAge(resp.Header.Get("Cache-Control")))
	return nil
}

// rsaKey membentuk kunci publik RSA dari modulus (n) dan eksponen (e) base64url.
func rsaKey(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	exp := new(big.Int).SetBytes(eb)
	if !exp.IsInt64() || exp.Int64() < 3 {
		return nil, errors.New("bad exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}, nil
}

// maxAge membaca max-age dari Cache-Control; tidak ada atau tidak valid = defaultKeyTTL. Dibatasi 1 menit sampai 24 jam.
func maxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "max-age="); ok {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				ttl := time.Duration(secs) * time.Second
				return min(max(ttl, time.Minute), 24*time.Hour)
			}
		}
	}
	return defaultKeyTTL
}
