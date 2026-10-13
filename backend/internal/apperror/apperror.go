package apperror

import "errors"

var (
	ErrNotFound             = errors.New("data not found")
	ErrDuplicate            = errors.New("duplicate data")
	ErrRetryExhausted       = errors.New("too many failed retries")
	ErrQueueEmpty           = errors.New("queue is currently empty")
	ErrSubscriptionInactive = errors.New("subscription is inactive")
	ErrQueueClosed          = errors.New("queue is closed")
	ErrAlreadyJoined        = errors.New("user already joined this queue")
)
