package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

func RequireAuth(jwtSecret string) gin.HandlerFunc {
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

		c.Set("user_id", userID)
		c.Next()
	}
}
