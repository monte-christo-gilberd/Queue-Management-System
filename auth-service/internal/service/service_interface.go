package service

import (
	"context"
	"time"

	"qms/auth-service/internal/model"
)

// UserRepository: akses tabel users. Dipenuhi repository.UserRepository.
type UserRepository interface {
	// Create menyimpan user baru; apperror.ErrDuplicate bila email sudah dipakai (username tidak UNIQUE).
	Create(
		ctx context.Context,
		email,
		username,
		passwordHash string,
	) (model.User, error)
	// FindByLogin mengembalikan user yang email atau username-nya sama dengan identifier.
	FindByLogin(
		ctx context.Context,
		identifier string,
	) ([]model.User, error)
	// FindByEmail: id, email, username untuk alur reset; apperror.ErrNotFound bila tidak ada.
	FindByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
	// FindForVerification: id, email, username, email_verified_at; apperror.ErrNotFound bila tidak ada.
	FindForVerification(
		ctx context.Context,
		userID int64,
	) (model.User, error)
	// PasswordChangedAt: nil bila belum pernah; apperror.ErrNotFound bila user tidak ada.
	PasswordChangedAt(
		ctx context.Context,
		userID int64,
	) (*time.Time, error)

	// Login Google.

	// FindByGoogleID: akun yang tertaut ke akun Google ini; apperror.ErrNotFound bila tidak ada.
	FindByGoogleID(
		ctx context.Context,
		googleID string,
	) (model.User, error)
	// FindAccountByEmail: akun lengkap (password_hash, google_id) dengan email ini, tanpa membedakan huruf besar atau
	// kecil; apperror.ErrNotFound bila tidak ada.
	FindAccountByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
	// FindByID: akun lengkap dengan id ini; apperror.ErrNotFound bila tidak ada.
	FindByID(
		ctx context.Context,
		id int64,
	) (model.User, error)
	// CreateGoogle membuat akun tanpa password dengan email yang sudah diverifikasi Google. apperror.ErrDuplicate bila
	// email atau google_id sudah dipakai.
	CreateGoogle(
		ctx context.Context,
		email,
		username,
		googleID string,
	) (model.User, error)
	// LinkGoogle menautkan akun Google ke akun yang belum tertaut dan menandai emailnya terverifikasi.
	// apperror.ErrDuplicate bila akun itu sudah tertaut ke akun Google lain, atau google_id sudah dipakai akun lain.
	LinkGoogle(
		ctx context.Context,
		userID int64,
		googleID string,
	) error
}

// TokenRepository: token sekali pakai (user_action_tokens). Dipenuhi repository.TokenRepository.
type TokenRepository interface {
	// CountRecentTokens menghitung token purpose ini milik user yang dibuat setelah since (batas kirim per jam).
	CountRecentTokens(
		ctx context.Context,
		userID int64,
		purpose string,
		since time.Time,
	) (int, error)
	// CreateToken menyimpan token baru dan membatalkan token lama (purpose sama) dalam satu transaksi.
	CreateToken(
		ctx context.Context,
		userID int64,
		purpose,
		tokenHash string,
		expiresAt time.Time,
		requestIP string,
	) error
	// ResetPassword memakai token, mengganti password, mengisi password_changed_at; ErrInvalidResetToken bila tidak sah.
	ResetPassword(
		ctx context.Context,
		tokenHash,
		passwordHash string,
	) error
	// VerifyEmail memakai token dan mengisi email_verified_at; ErrInvalidResetToken bila tidak sah. Mengembalikan id
	// pemilik token dan newlyVerified (true bila akun baru terverifikasi oleh token ini).
	VerifyEmail(
		ctx context.Context,
		tokenHash string,
	) (userID int64, newlyVerified bool, err error)
}

// Mailer mengirim email lewat mail-service (mailclient.Client).
type Mailer interface {
	Send(
		ctx context.Context,
		to,
		template string,
		data map[string]any,
	) error
}

// GoogleVerifier memeriksa ID token Google. Dipenuhi google.Verifier; di tes diganti fake.
type GoogleVerifier interface {
	Verify(
		ctx context.Context,
		idToken string,
	) (model.GoogleIdentity, error)
}
