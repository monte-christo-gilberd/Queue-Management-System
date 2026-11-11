package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/apperror"
)

// Authenticator memeriksa token login dan pencabutan sesi, lalu mengembalikan id user (dipenuhi service.Service).
type Authenticator interface {
	// Authenticate mengembalikan id user; apperror.ErrUnauthenticated bila token tidak sah atau sesi dicabut.
	Authenticate(
		ctx context.Context,
		raw string,
	) (int64, error)
}

// Auth memeriksa JWT RS256 dan pencabutan sesi, lalu menyimpan user_id di context.
func Auth(
	authenticator Authenticator,
) gin.HandlerFunc {
	// logger untuk galat server yang tidak dikirim ke klien
	log := slog.Default()
	// kembalikan handler Gin yang memeriksa header Authorization
	return func(c *gin.Context) {
		// baca header Authorization; tanpa awalan "Bearer " request ditolak
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
			return
		}
		// periksa token dan pencabutan sesi; galat dipetakan ke status di bawah
		userID, err := authenticator.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		switch {
		// token tidak sah, kedaluwarsa, atau sesi dicabut: 401
		case errors.Is(err, apperror.ErrUnauthenticated):
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
			return
		// gangguan database: 500, bukan salah klien
		case err != nil:
			log.Error("authenticate failed", "err", err)
			abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}
		// token sah: simpan id user di context agar handler bisa membacanya, lalu teruskan request
		c.Set("user_id", userID)
		c.Next()
	}
}
