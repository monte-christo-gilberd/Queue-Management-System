package service

import (
	"qms/qr-service/internal/model"
	"time"
)

// TokenServiceInterface: kontrak service token join yang dipakai handler dan router. Dipenuhi *TokenService.
type TokenServiceInterface interface {
	IssueToken(
		queueID int64,
		typ string,
		ttlSeconds int,
	) (string, model.Claims, error)
	Issue(
		queueID int64,
		typ model.Type,
		ttl time.Duration,
	) (string, model.Claims, error)
	VerifyToken(
		token string,
	) (model.Claims, error)
}

// RenderServiceInterface: kontrak service render QR yang dipakai handler dan router. Dipenuhi *RenderService.
type RenderServiceInterface interface {
	Render(
		opts model.RenderOptions,
	) ([]byte, error)
}
