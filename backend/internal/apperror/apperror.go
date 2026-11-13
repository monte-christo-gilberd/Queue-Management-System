package apperror

import "errors"

var (
	// 1. Tidak ditemukan / duplikat: data tidak ada, sudah ada, atau identifier merujuk lebih dari satu akun.
	ErrNotFound             = errors.New("data not found")
	ErrDuplicate            = errors.New("duplicate data")
	ErrDuplicateCounterName = errors.New("a counter with this name already exists")
	ErrAmbiguousAccount     = errors.New("account identifier matches more than one account")

	// 2. Input tidak valid: data dari client tidak memenuhi aturan format, rentang, atau ukuran.
	ErrInvalidNote          = errors.New("invalid note")
	ErrInvalidPosition      = errors.New("position must be at least 1")
	ErrInvalidCounterName   = errors.New("counter name must be 1-30 characters")
	ErrInvalidUsername      = errors.New("username must be 1-50 characters")
	ErrInvalidImage         = errors.New("invalid image file")
	ErrImageTooLarge        = errors.New("image file is larger than 5 MB")
	ErrUnsupportedImage     = errors.New("image must be JPEG, PNG or WebP")
	ErrInvalidCursor        = errors.New("invalid pagination cursor")
	ErrInvalidDisplayConfig = errors.New("display counts must be between 0 and 10")
	ErrInvalidSkipSettings  = errors.New("invalid skip settings")
	ErrWeakPassword         = errors.New("password must be 8 to 72 characters")

	// 3. Aturan state antrian/tiket: aksi valid secara format, tetapi tidak bisa dilakukan karena keadaan saat ini.
	ErrQueueEmpty      = errors.New("queue is currently empty")
	ErrNothingToSkip   = errors.New("no ticket is being served")
	ErrNothingToRecall = errors.New("no ticket is being served")
	ErrRecallTooSoon   = errors.New("recall not available yet")
	ErrAlreadyRecalled = errors.New("ticket already recalled")
	ErrQueueClosed     = errors.New("queue is closed")
	ErrAlreadyJoined   = errors.New("user already joined this queue")

	// 4. Konflik dan batas kuota: perubahan bertabrakan dengan keadaan terbaru, atau melewati batas yang diizinkan.
	ErrCounterStateChanged = errors.New("counter state changed since the device last read it")
	ErrCounterLimit        = errors.New("a queue can have at most 10 counters")
	ErrDefaultCounter      = errors.New("the default counter cannot be deleted")
	ErrTooManyDisplays     = errors.New("too many public displays are connected to this queue")
	ErrTooManyVerifyEmails = errors.New("too many verification emails requested, try again later")

	// 5. Langganan / izin: owner tidak punya langganan aktif, atau email akun belum diverifikasi.
	ErrSubscriptionInactive = errors.New("subscription is inactive")
	ErrEmailNotVerified     = errors.New("email is not verified")

	// 6. Autentikasi token dan QR: token atau kode tidak valid, kedaluwarsa, atau tidak boleh dipakai untuk aksi itu.
	// ErrInvalidResetToken dipakai bersama oleh reset password dan verifikasi email (token sekali pakai yang sama bentuknya).
	ErrInvalidResetToken   = errors.New("reset token is invalid, expired or already used")
	ErrQRInvalid           = errors.New("qr token invalid")
	ErrQRExpired           = errors.New("qr token expired")
	ErrQRTypeInvalid       = errors.New("qr type invalid")
	ErrQRReplaceNotAllowed = errors.New("qr replacement not allowed")

	// 7. Kegagalan layanan / infrastruktur: layanan eksternal tidak bisa dihubungi atau retry sudah habis (kesalahan server).
	ErrQRUnavailable = errors.New("qr service unavailable")

	// ErrPaymentUnavailable: payment-service tidak bisa dihubungi atau gagal sementara (503).
	ErrPaymentUnavailable = errors.New("payment service unavailable")
	// ErrPaymentRejected: payment-service menolak permintaan pembayaran (502).
	ErrPaymentRejected = errors.New("payment request rejected")
	// ErrInvalidPaymentMethod: metode pembayaran tidak didukung, hanya qris (400).
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrRetryExhausted       = errors.New("too many failed retries")
)
