package apperror

import "errors"

var (
	ErrNotFound             = errors.New("data not found")
	ErrDuplicate            = errors.New("duplicate data")
	ErrRetryExhausted       = errors.New("too many failed retries")
	ErrQueueEmpty           = errors.New("queue is currently empty")
	ErrNothingToSkip        = errors.New("no ticket is being served")
	ErrInvalidSkipSettings  = errors.New("invalid skip settings")
	ErrSubscriptionInactive = errors.New("subscription is inactive")
	ErrQueueClosed          = errors.New("queue is closed")
	ErrAlreadyJoined        = errors.New("user already joined this queue")

	// Error QR: token join tidak valid/tidak ada atau qr-service gagal dipakai.
	ErrQRInvalid           = errors.New("qr token invalid")
	ErrQRExpired           = errors.New("qr token expired")
	ErrQRTypeInvalid       = errors.New("qr type invalid")
	ErrQRReplaceNotAllowed = errors.New("qr replacement not allowed")
	ErrQRUnavailable       = errors.New("qr service unavailable")
)
