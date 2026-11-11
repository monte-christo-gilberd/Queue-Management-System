package apperror

import "errors"

var (
	// ErrNotFound: data tidak ada (akun tidak ditemukan, atau kredensial tidak cocok).
	ErrNotFound = errors.New("data not found")
	// ErrDuplicate: email atau username sudah dipakai (pelanggaran UNIQUE).
	ErrDuplicate = errors.New("duplicate data")
	// ErrWeakPassword: password kurang dari 8 atau lebih dari 72 karakter/byte.
	ErrWeakPassword = errors.New("password must be 8 to 72 characters")
	// ErrTooManyVerifyEmails: batas kirim email verifikasi per akun per jam sudah habis.
	ErrTooManyVerifyEmails = errors.New("too many verification emails requested, try again later")
	// ErrInvalidResetToken: dipakai bersama reset password dan verifikasi email (token sekali pakai).
	ErrInvalidResetToken = errors.New("reset token is invalid, expired or already used")
	// ErrUnauthenticated: token tidak sah, kedaluwarsa, atau sesinya sudah dicabut.
	ErrUnauthenticated = errors.New("unauthenticated")

	// Login Google.

	// ErrGoogleInvalidToken: ID token Google tidak sah (tanda tangan, issuer, audience, atau kedaluwarsa).
	ErrGoogleInvalidToken = errors.New("invalid google id token")
	// ErrGoogleEmailNotVerified: Google menyatakan email akun itu belum terverifikasi.
	ErrGoogleEmailNotVerified = errors.New("google email is not verified")
	// ErrGoogleUnavailable: kunci publik Google tidak bisa diambil, jadi token tidak bisa diperiksa.
	ErrGoogleUnavailable = errors.New("google verification unavailable")
	// ErrGoogleLinkRequired: email sudah terdaftar dengan password; akun Google hanya boleh ditautkan setelah user
	// membuktikan password-nya. Dibungkus service.LinkRequiredError yang membawa link token.
	ErrGoogleLinkRequired = errors.New("google link required")
	// ErrInvalidLinkToken: link token tidak sah, kedaluwarsa, salah tujuan, atau sudah terlalu sering salah password.
	ErrInvalidLinkToken = errors.New("invalid link token")
)
