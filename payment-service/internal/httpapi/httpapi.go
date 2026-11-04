// Package httpapi: API HTTP payment-service. Kontrak respons sama dengan backend QMS: sukses
// {"data": ...}, gagal {"error": {"code", "message"}}.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"

	"qms/payment-service/internal/billing"
)

// maxWebhookBody membatasi payload webhook yang dibaca.
const maxWebhookBody = 1 << 20 // 1 MiB

// Billing: yang dibutuhkan API dari lapisan bisnis (dan yang bisa diganti di test).
type Billing interface {
	Health(ctx context.Context) error
	StartCheckout(ctx context.Context, ownerID int64) (billing.Checkout, error)
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error
}

type API struct {
	svc       Billing
	jwtSecret []byte
	log       *slog.Logger
}

func New(svc Billing, jwtSecret string, log *slog.Logger) *API {
	return &API{svc: svc, jwtSecret: []byte(jwtSecret), log: log}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /api/v1/payments/checkout", a.checkout)
	mux.HandleFunc("POST /api/v1/payments/webhooks/stripe", a.webhook)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if err := a.svc.Health(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) checkout(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	co, err := a.svc.StartCheckout(ctx, ownerID)
	switch {
	case errors.Is(err, billing.ErrNotOwner):
		writeError(w, http.StatusForbidden, "NOT_OWNER", "only owners can renew a subscription")
	case errors.Is(err, billing.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "PAYMENT_UNAVAILABLE", "payments are temporarily unavailable")
	case err != nil:
		a.log.Error("start checkout failed", "owner_id", ownerID, "err", err)
		writeError(w, http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR", "payment provider error, try again later")
	default:
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{
			"checkout_url": co.URL,
			"expires_at":   co.ExpiresAt.Format(time.RFC3339),
		}})
	}
}

func (a *API) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "could not read body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	err = a.svc.HandleWebhook(ctx, body, r.Header.Get("Stripe-Signature"))
	switch {
	case errors.Is(err, billing.ErrBadSignature):
		a.log.Warn("stripe webhook signature invalid", "remote", r.RemoteAddr, "err", err)
		writeError(w, http.StatusBadRequest, "BAD_SIGNATURE", "invalid webhook signature")
	case errors.Is(err, billing.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "PAYMENT_UNAVAILABLE", "payments are not configured")
	case err != nil:
		// 5xx membuat Stripe mengirim ulang; perpanjangan terkejar saat penyimpanan pulih.
		a.log.Error("webhook handling failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
	default:
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"received": true}})
	}
}

// authenticate memverifikasi JWT QMS (HS256, JWT_SECRET sama dengan backend) dan mengembalikan id user.
func (a *API) authenticate(r *http.Request) (int64, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return 0, false
	}
	tok, err := jwt.Parse(strings.TrimPrefix(h, "Bearer "), func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil || !tok.Valid {
		return 0, false
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
