package handler

import (
	"context"
	"errors"
	"net/http"

	"qms/internal/apperror"
	"qms/internal/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionCheckoutService: logika checkout langganan (dipenuhi *service.SubscriptionCheckout).
type SubscriptionCheckoutService interface {
	Checkout(ctx context.Context, ownerID int64, in service.CheckoutInput) (service.CheckoutResult, error)
}

// SubscriptionCheckoutHandler menangani POST /api/v1/owners/subscription/checkout.
type SubscriptionCheckoutHandler struct {
	service SubscriptionCheckoutService
}

// NewSubscriptionCheckoutHandler membuat handler checkout langganan.
func NewSubscriptionCheckoutHandler(s SubscriptionCheckoutService) *SubscriptionCheckoutHandler {
	return &SubscriptionCheckoutHandler{service: s}
}

// checkoutBody: body JSON dari frontend.
type checkoutBody struct {
	Method    string `json:"method"`
	BankCode  string `json:"bank_code"`
	ReturnURL string `json:"return_url"`
}

// Checkout: membuat pesanan pembayaran langganan dan mengembalikan pay_url.
func (h *SubscriptionCheckoutHandler) Checkout(c *gin.Context) {
	var body checkoutBody
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	res, err := h.service.Checkout(c.Request.Context(), c.GetInt64("user_id"), service.CheckoutInput{
		Method:    body.Method,
		BankCode:  body.BankCode,
		ReturnURL: body.ReturnURL,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrEmailNotVerified):
			respondError(c, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "verify your email before subscribing")
		case errors.Is(err, apperror.ErrInvalidPaymentMethod):
			respondError(c, http.StatusBadRequest, "INVALID_PAYMENT_METHOD", "method must be qris, va, or ewallet; va and ewallet need bank_code")
		case errors.Is(err, apperror.ErrPaymentUnavailable):
			respondError(c, http.StatusServiceUnavailable, "PAYMENT_UNAVAILABLE", "payment service is unavailable")
		case errors.Is(err, apperror.ErrPaymentRejected):
			respondError(c, http.StatusBadGateway, "PAYMENT_REJECTED", "payment request was rejected")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}

	respondData(c, http.StatusOK, gin.H{
		"reference_id": res.ReferenceID,
		"pay_url":      res.PayURL,
		"expires_at":   res.ExpiresAt,
		"amount":       res.Amount,
	})
}
