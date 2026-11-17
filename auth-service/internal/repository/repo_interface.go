package repository

import (
	"context"
	"time"

	"qms/auth-service/internal/model"
)

// UserRepositoryInterface menggambarkan bentuk UserRepository (tabel users). Dipakai terutama oleh tes untuk
// membuat fake dalam memori.
type UserRepositoryInterface interface {
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
	// FindByEmail mengembalikan id, email, username untuk alur reset.
	FindByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
	// FindForVerification mengembalikan id, email, username, email_verified_at.
	FindForVerification(
		ctx context.Context,
		userID int64,
	) (model.User, error)
	// PasswordChangedAt mengembalikan waktu password terakhir diganti (nil bila belum pernah).
	PasswordChangedAt(
		ctx context.Context,
		userID int64,
	) (*time.Time, error)
	// FindByGoogleID mengembalikan akun yang tertaut ke akun Google ini.
	FindByGoogleID(
		ctx context.Context,
		googleID string,
	) (model.User, error)
	// FindAccountByEmail mengembalikan akun lengkap dengan email ini, tanpa membedakan huruf besar atau kecil.
	FindAccountByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
	// FindByID mengembalikan akun lengkap dengan id ini.
	FindByID(
		ctx context.Context,
		id int64,
	) (model.User, error)
	// CreateGoogle membuat akun tanpa password dengan email yang sudah diverifikasi Google.
	CreateGoogle(
		ctx context.Context,
		email,
		username,
		googleID string,
	) (model.User, error)
	// LinkGoogle menautkan akun Google ke akun yang belum tertaut.
	LinkGoogle(
		ctx context.Context,
		userID int64,
		googleID string,
	) error
}

// TokenRepositoryInterface menggambarkan bentuk TokenRepository (tabel user_action_tokens).
type TokenRepositoryInterface interface {
	// CountRecentTokens menghitung token purpose ini milik user yang dibuat setelah since.
	CountRecentTokens(
		ctx context.Context,
		userID int64,
		purpose string,
		since time.Time,
	) (int, error)
	// CreateToken menyimpan token baru dan mencabut token lama (purpose sama) dalam satu transaksi.
	CreateToken(
		ctx context.Context,
		userID int64,
		purpose,
		tokenHash string,
		expiresAt time.Time,
		requestIP string,
	) error
	// ResetPassword memakai token reset dan mengganti password secara atomik.
	ResetPassword(
		ctx context.Context,
		tokenHash,
		passwordHash string,
	) error
	// VerifyEmail memakai token verifikasi dan mengisi email_verified_at secara atomik. Mengembalikan id pemilik
	// token dan newlyVerified (true bila akun baru terverifikasi oleh token ini, bukan sebelumnya).
	VerifyEmail(
		ctx context.Context,
		tokenHash string,
	) (userID int64, newlyVerified bool, err error)
}

// Pastikan implementasi nyata memenuhi interface di atas (diperiksa saat kompilasi).
var (
	_ UserRepositoryInterface  = (*UserRepository)(nil)
	_ TokenRepositoryInterface = (*TokenRepository)(nil)
)
