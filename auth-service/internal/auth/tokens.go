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

const (
	// linkTokenTTL: masa berlaku link token Google. Pendek, karena hanya menjembatani layar konfirmasi password.
	linkTokenTTL = 5 * time.Minute
	// purposeGoogleLink: nilai klaim `purpose` pada link token. Token login TIDAK punya klaim ini.
	purposeGoogleLink = "google_link"
)

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

// IssueLink membuat link token: JWT RS256 pendek (5 menit) yang mengikat user id dan google id untuk layar
// konfirmasi password. Memuat klaim `purpose`, sehingga TIDAK diterima sebagai token login oleh Verify di sini,
// middleware backend, maupun payment-service (kunci penandatangannya sama dengan token login).
func (t *TokenIssuer) IssueLink(userID int64, googleID string) (string, error) {
	// jti acak: tiap link token unik. Tanpa ini, token untuk akun dan Google ID yang sama dalam detik yang sama
	// identik, sehingga token baru ikut "terbakar" bersama token lama (penghitung percobaan salah password).
	jti, err := newActionToken()
	if err != nil {
		return "", err
	}
	now := t.now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":     strconv.FormatInt(userID, 10),
		"gid":     googleID,
		"purpose": purposeGoogleLink,
		"jti":     jti,
		"iat":     now.Unix(),
		"exp":     now.Add(linkTokenTTL).Unix(),
	})
	token.Header["kid"] = t.kid
	return token.SignedString(t.key)
}

// VerifyLink memeriksa link token dan mengembalikan user id dan google id. Token login (tanpa `purpose`) ditolak.
func (t *TokenIssuer) VerifyLink(raw string) (int64, string, error) {
	claims, err := t.parse(raw)
	if err != nil {
		return 0, "", err
	}
	if p, _ := claims["purpose"].(string); p != purposeGoogleLink {
		return 0, "", errors.New("not a link token")
	}
	sub, _ := claims["sub"].(string)
	userID, err := strconv.ParseInt(sub, 10, 64)
	gid, _ := claims["gid"].(string)
	if err != nil || userID <= 0 || gid == "" {
		return 0, "", errors.New("invalid link claims")
	}
	return userID, gid, nil
}

// Verify memeriksa tanda tangan dan masa berlaku token LOGIN. Hanya RS256 yang diterima; algoritma lain (HS256,
// none) ditolak supaya tidak ada serangan algorithm confusion. Token yang punya klaim `purpose` (mis. link token)
// bukan token login dan ditolak.
func (t *TokenIssuer) Verify(raw string) (jwt.MapClaims, error) {
	claims, err := t.parse(raw)
	if err != nil {
		return nil, err
	}
	if _, has := claims["purpose"]; has {
		return nil, errors.New("not a login token")
	}
	return claims, nil
}

// parse memeriksa tanda tangan RS256 dan masa berlaku, tanpa memeriksa tujuan token.
func (t *TokenIssuer) parse(raw string) (jwt.MapClaims, error) {
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
