package middleware

import (
	"context"
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

// RequireAuth memeriksa JWT saja (tanpa akses database).
// Dipakai di tes. Produksi memakai RequireAuthWithRevocation (lihat cmd/server/main.go).
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return RequireAuthWithRevocation(jwtSecret, nil)
}

// RequireAuthWithRevocation seperti RequireAuth, ditambah pencabutan sesi: token yang terbit (klaim `iat`, waktu
// token dibuat dalam detik Unix) sebelum password diganti ditolak. Token tanpa `iat` (terbitan sebelum fitur ini) tetap sah sampai `exp` dan tidak
// memicu query. Biayanya satu query per request untuk token ber-`iat`; diukur di item 13.
//
// Hasilnya fungsi middleware: bila token valid, user_id disimpan di context lalu request diteruskan; bila tidak,
// request dihentikan dengan status 401 (atau 500 bila pengecekan pencabutan gagal karena database).
func RequireAuthWithRevocation(jwtSecret string, checker SessionRevocationChecker) gin.HandlerFunc {
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

		// memverifikasi tanda tangan dan masa berlaku token memakai JWT_SECRET
		jwtToken, err := jwt.Parse(
			token, func(t *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
		if err != nil {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"invalid jwt token",
			)
			return
		}

		// membaca isi token (claims); gagal bila bentuknya tidak dikenali
		claims, ok := jwtToken.Claims.(jwt.MapClaims)
		if !ok {
			abortError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"invalid token claims",
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
		// Klaim ini diisi di service/user_service.go saat login. JSON mengubah angka menjadi float64, jadi dibaca sebagai float64.
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
