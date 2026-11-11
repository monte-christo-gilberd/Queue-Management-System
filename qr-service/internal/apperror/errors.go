package apperror

import "errors"

var (
	// 1. Error token: hasil verifikasi atau pembuatan token join yang gagal.
	// ErrTokenMalformed: token tidak punya tiga bagian, panjangnya salah, atau base64 rusak.
	ErrTokenMalformed = errors.New("malformed token")
	// ErrTokenInvalidSignature: tanda tangan tidak cocok dengan isi token dan secret.
	ErrTokenInvalidSignature = errors.New("invalid token signature")
	// ErrTokenExpired: waktu sekarang sudah melewati ExpiresAt token.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenUnsupportedFormat: versi di depan token bukan versi yang dikenal.
	ErrTokenUnsupportedFormat = errors.New("unsupported token version")
	// ErrInvalidPayload: isi klaim tidak valid (id antrian, jenis, JTI, atau exp).
	ErrInvalidPayload = errors.New("invalid token payload")

	// 2. Input tidak valid: data dari client tidak memenuhi aturan.
	// ErrInvalidQueueID: queue_id harus bilangan positif.
	ErrInvalidQueueID = errors.New("queue_id must be positive")
	// ErrInvalidTokenType: jenis token bukan static atau dynamic.
	ErrInvalidTokenType = errors.New("type must be static or dynamic")
	// ErrTTLOutOfRange: ttl_seconds token dinamis di luar rentang 30 detik sampai 10 menit.
	ErrTTLOutOfRange = errors.New("ttl_seconds out of range")
	// ErrInvalidRenderData: data QR kosong atau lebih dari 1024 karakter.
	ErrInvalidRenderData = errors.New("data is required (max 1024 characters)")
)
