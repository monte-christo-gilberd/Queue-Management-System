package auth

import (
	"crypto/rsa"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
)

// tokenTTL: masa berlaku token login. Sama dengan yang dulu dipakai backend (24 jam).
const tokenTTL = 24 * time.Hour

// TokenIssuer menerbitkan dan memeriksa JWT RS256. Kunci privat hanya ada di auth-service; backend memverifikasi
// dengan kunci publik sehingga tidak perlu menghubungi layanan ini saat memeriksa token.
type TokenIssuer struct {
	// key: kunci privat RSA untuk menandatangani token. Kunci publiknya dipakai Verify.
	key *rsa.PrivateKey
	// kid: nilai header `kid` pada token, penanda kunci untuk rotasi kunci di kemudian hari.
	kid string
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
}

// NewTokenIssuer membuat TokenIssuer dengan kunci privat dan nilai header `kid`.
func NewTokenIssuer(key *rsa.PrivateKey, kid string) *TokenIssuer {
	// waktu memakai time.Now; tes mengganti field now bila perlu
	return &TokenIssuer{key: key, kid: kid, now: time.Now}
}

// Issue membuat JWT RS256 dengan klaim sub (id user), iat (waktu terbit, dipakai pencabutan sesi), dan exp (24 jam).
func (t *TokenIssuer) Issue(userID int64) (string, error) {
	// waktu sekarang menjadi dasar klaim iat dan exp
	now := t.now()
	// susun token: sub berisi ID user sebagai string (format yang dibaca middleware backend)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		// iat dipakai pencabutan sesi: token yang terbit sebelum password diganti ditolak
		"iat": now.Unix(),
		"exp": now.Add(tokenTTL).Unix(),
	})
	// pasang penanda kunci di header
	token.Header["kid"] = t.kid
	// tandatangani dengan kunci privat dan kembalikan string token
	return token.SignedString(t.key)
}

// Verify memeriksa tanda tangan dan masa berlaku. Hanya RS256 yang diterima; algoritma lain (HS256, none)
// ditolak supaya tidak ada serangan algorithm confusion.
func (t *TokenIssuer) Verify(raw string) (jwt.MapClaims, error) {
	// jwt.Parse memeriksa tanda tangan dan exp; fungsi kunci memastikan algoritmanya RS256
	parsed, err := jwt.Parse(raw, func(tok *jwt.Token) (interface{}, error) {
		// tolak apa pun selain RS256, termasuk HS256 yang memakai kunci publik sebagai secret
		if tok.Method != jwt.SigningMethodRS256 {
			return nil, errors.New("unexpected signing method")
		}
		return &t.key.PublicKey, nil
	})
	// token rusak, kedaluwarsa, atau tanda tangan salah
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	// klaim dibaca sebagai map; bentuk lain dianggap tidak sah
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}
