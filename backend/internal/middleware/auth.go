package middleware

import (
	"context"
	"crypto/rsa"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

// SessionRevocationChecker memberi waktu terakhir password user diganti (nil bila belum pernah).
// apperror.ErrNotFound bila user sudah tidak ada.
type SessionRevocationChecker interface {
	PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error)
}

// TokenVerifier memeriksa tanda tangan dan masa berlaku JWT secara lokal, tanpa panggilan jaringan. Token
// diterbitkan auth-service dengan RS256; backend hanya memegang kunci publik, jadi bila auth-service mati token
// yang sudah terbit tetap bisa diverifikasi.
type TokenVerifier struct {
	pub    *rsa.PublicKey
	legacy []byte
}

// NewTokenVerifier membuat TokenVerifier. legacyHS256Secret boleh kosong; bila diisi, token HS256 lama (terbitan
// sebelum auth-service dipisah) tetap diterima sampai kedaluwarsa. Hapus setelah 24 jam.
func NewTokenVerifier(pub *rsa.PublicKey, legacyHS256Secret string) *TokenVerifier {
	v := &TokenVerifier{pub: pub}
	if legacyHS256Secret != "" {
		v.legacy = []byte(legacyHS256Secret)
	}
	return v
}

// Verify mengembalikan klaim token yang sah. Algoritma ditegakkan per kunci: RS256 hanya dengan kunci publik,
// HS256 hanya dengan secret lama. Ini mencegah algorithm confusion (HS256 yang ditandatangani memakai kunci publik
// sebagai secret) dan menolak algoritma `none`.
func (v *TokenVerifier) Verify(raw string) (jwt.MapClaims, error) {
	parsed, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		switch t.Method {
		case jwt.SigningMethodRS256:
			return v.pub, nil
		case jwt.SigningMethodHS256:
			if len(v.legacy) > 0 {
				return v.legacy, nil
			}
		}
		return nil, errors.New("unexpected signing method")
	})
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// RequireAuth memeriksa JWT saja (tanpa akses database).
// Dipakai di tes. Produksi memakai RequireAuthWithRevocation (lihat cmd/server/main.go).
func RequireAuth(verifier *TokenVerifier) gin.HandlerFunc {
	return RequireAuthWithRevocation(verifier, nil)
}

// RequireAuthWithRevocation seperti RequireAuth, ditambah pencabutan sesi: token yang terbit (klaim `iat`, waktu
// token dibuat dalam detik Unix) sebelum password diganti ditolak. Token tanpa `iat` (terbitan sebelum fitur ini) tetap sah sampai `exp` dan tidak
// memicu query. Biayanya satu query per request untuk token ber-`iat`; diukur di item 13.
//
// Hasilnya fungsi middleware: bila token valid, user_id disimpan di context lalu request diteruskan; bila tidak,
// request dihentikan dengan status 401 (atau 500 bila pengecekan pencabutan gagal karena database).
func RequireAuthWithRevocation(verifier *TokenVerifier, checker SessionRevocationChecker) gin.HandlerFunc {
	return func(c *gin.Context) {

		// membaca header Authorization; tanpa prefix "Bearer" request ditolak
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer") {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"unauthorized user",
			)
			return
		}

		// mengambil token dari header dengan membuang prefix "Bearer "
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// memverifikasi tanda tangan dan masa berlaku token secara lokal (kunci publik RS256)
		claims, err := verifier.Verify(token)
		if err != nil {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"invalid jwt token",
			)
			return
		}

		// membaca klaim `sub` (ID user) yang harus berupa string
		sub, ok := claims["sub"].(string)
		if !ok {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"invalid subject claim",
			)
			return
		}

		// mengubah `sub` menjadi ID user berupa angka
		userID, err := strconv.ParseInt(sub, 10, 64)
		if err != nil {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"invalid subject claim",
			)
			return
		}

		// pencabutan sesi: hanya untuk token yang punya klaim `iat` dan checker sudah dipasang
		// iat (issued at) adalah waktu token diterbitkan, dalam detik sejak 1 Januari 1970 (Unix time).
		// Klaim ini diisi auth-service saat login. JSON mengubah angka menjadi float64, jadi dibaca sebagai float64.
		if iat, ok := claims["iat"].(float64); ok && checker != nil {

			// mengambil waktu terakhir password user diganti dari database
			changedAt, err := checker.PasswordChangedAt(c.Request.Context(), userID)

			// user sudah tidak ada: sesinya dianggap tidak sah
			if errors.Is(err, apperror.ErrNotFound) {
				abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
				return
			}

			// gangguan database: server error, bukan salah klien
			if err != nil {
				abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				return
			}

			// Dibandingkan dalam detik: iat berdetik, dan login tepat setelah reset tidak boleh ikut dicabut.
			// token yang terbit sebelum password diganti dicabut
			if changedAt != nil && changedAt.Unix() > int64(iat) {
				abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "session revoked, please log in again")
				return
			}
		}

		// token valid: menyimpan ID user di context agar handler bisa membacanya, lalu meneruskan request
		c.Set("user_id", userID)
		c.Next()
	}
}
