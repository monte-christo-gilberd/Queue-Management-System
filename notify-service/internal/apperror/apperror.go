package apperror

import "errors"

// Galat sentinel notify-service. Pemanggil membandingkannya dengan errors.Is.
var (
	// 1. Langganan push: hasil kirim yang menentukan tindakan berikutnya.
	// ErrSubscriptionGone: langganan sudah tidak berlaku (HTTP 404/410 dari push service).
	ErrSubscriptionGone = errors.New("subscription gone")
	// ErrTemporaryPush: kegagalan sementara (jaringan, HTTP 429/5xx) yang layak dicoba ulang.
	ErrTemporaryPush = errors.New("temporary push failure")
)
