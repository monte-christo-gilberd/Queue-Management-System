package joinlink

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// Version adalah versi format token. Naikkan bila format berubah.
	Version = "v1"

	// MaxTokenLength membatasi token yang diterima agar payload raksasa ditolak lebih awal.
	MaxTokenLength = 1024

	maxPayloadBytes = 512

	// DefaultDynamicTTL dipakai bila pemanggil tidak menentukan TTL untuk token dinamis.
	DefaultDynamicTTL = 120 * time.Second
)

var (
	ErrMalformed         = errors.New("malformed token")
	ErrInvalidSignature  = errors.New("invalid token signature")
	ErrExpired           = errors.New("token expired")
	ErrUnsupportedFormat = errors.New("unsupported token version")
	ErrInvalidPayload    = errors.New("invalid token payload")
)

// Type adalah jenis kode QR join.
type Type string

const (
	TypeStatic  Type = "static"
	TypeDynamic Type = "dynamic"
)

func (t Type) Valid() bool {
	return t == TypeStatic || t == TypeDynamic
}

// Claims adalah isi token join. Token statis tidak punya ExpiresAt.
type Claims struct {
	QueueID   int64  `json:"qid"`
	Type      Type   `json:"typ"`
	JTI       string `json:"jti"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

// Signer membuat dan memverifikasi token join bertanda tangan HMAC-SHA256.
// Token tidak menyimpan data rahasia; tanda tangan mencegah pemalsuan.
type Signer struct {
	secret []byte
	now    func() time.Time
}

// New membuat Signer. Secret minimal 32 byte agar tidak mudah ditebak.
func New(secret string) (*Signer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("join link secret must be at least 32 bytes")
	}
	return &Signer{secret: []byte(secret), now: time.Now}, nil
}

// Issue membuat token untuk satu antrian.
// Token statis tidak punya exp; token dinamis memakai ttl (default DefaultDynamicTTL).
func (s *Signer) Issue(queueID int64, typ Type, ttl time.Duration) (string, Claims, error) {
	if queueID <= 0 {
		return "", Claims{}, ErrInvalidPayload
	}
	if !typ.Valid() {
		return "", Claims{}, ErrInvalidPayload
	}

	jti, err := randomID()
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate jti: %w", err)
	}

	claims := Claims{
		QueueID: queueID,
		Type:    typ,
		JTI:     jti,
	}
	if typ == TypeDynamic {
		if ttl <= 0 {
			ttl = DefaultDynamicTTL
		}
		claims.ExpiresAt = s.now().Add(ttl).Unix()
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", Claims{}, fmt.Errorf("marshal claims: %w", err)
	}
	if len(payload) > maxPayloadBytes {
		return "", Claims{}, ErrInvalidPayload
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := Version + "." + payloadB64
	token := signingInput + "." + s.sign(signingInput)

	return token, claims, nil
}

// Verify memeriksa format, tanda tangan, dan masa berlaku token.
func (s *Signer) Verify(token string) (Claims, error) {
	if token == "" || len(token) > MaxTokenLength {
		return Claims{}, ErrMalformed
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformed
	}
	if parts[0] != Version {
		return Claims{}, ErrUnsupportedFormat
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(payload) == 0 || len(payload) > maxPayloadBytes {
		return Claims{}, ErrMalformed
	}

	expected := s.sign(parts[0] + "." + parts[1])
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrInvalidSignature
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrMalformed
	}
	if claims.QueueID <= 0 || !claims.Type.Valid() || claims.JTI == "" {
		return Claims{}, ErrInvalidPayload
	}
	if claims.Type == TypeDynamic && claims.ExpiresAt == 0 {
		return Claims{}, ErrInvalidPayload
	}
	if claims.ExpiresAt > 0 && s.now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrExpired
	}

	return claims, nil
}

func (s *Signer) sign(signingInput string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
