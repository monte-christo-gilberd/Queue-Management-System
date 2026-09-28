package apperror

import "errors"

var (
	ErrNotFound         = errors.New("data not found")
	ErrDuplicate        = errors.New("duplicate data")
	ErrRetryExhausted   = errors.New("too many failed retries")
	ErrQueueEmpty       = errors.New("queue is currently empty")
	ErrBadRequest       = errors.New("invalid input")
	ErrInternalDatabase = errors.New("Internal database error")
)
