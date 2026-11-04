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
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return RequireAuthWithRevocation(jwtSecret, nil)
}

// RequireAuthWithRevocation seperti RequireAuth, ditambah pencabutan sesi: token yang terbit (klaim `iat`) sebelum
// password diganti ditolak. Token tanpa `iat` (terbitan sebelum fitur ini) tetap sah sampai `exp` dan tidak
// memicu query. Biayanya satu query per request untuk token ber-`iat`; diukur di item 13.
func RequireAuthWithRevocation(jwtSecret string, checker SessionRevocationChecker) gin.HandlerFunc {
	return func(c *gin.Context) {

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
		token := strings.TrimPrefix(authHeader, "Bearer ")

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

		if iat, ok := claims["iat"].(float64); ok && checker != nil {
			changedAt, err := checker.PasswordChangedAt(c.Request.Context(), userID)
			if errors.Is(err, apperror.ErrNotFound) {
				abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
				return
			}
			if err != nil {
				abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				return
			}
			// Dibandingkan dalam detik: iat berdetik, dan login tepat setelah reset tidak boleh ikut dicabut.
			if changedAt != nil && changedAt.Unix() > int64(iat) {
				abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "session revoked, please log in again")
				return
			}
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
