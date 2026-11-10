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
)
