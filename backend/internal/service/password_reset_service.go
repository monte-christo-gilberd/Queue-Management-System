package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"qms/internal/apperror"
	"qms/internal/model"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// PurposePasswordReset: nilai kolom user_action_tokens.purpose untuk token reset password.
	PurposePasswordReset = "password_reset"

	resetTokenTTL = 30 * time.Minute
	// Batas per akun, di atas batas per IP di nginx: satu orang tidak bisa membanjiri kotak masuk korban.
	resetMaxPerHour = 3

	minPasswordRunes = 8
	// bcrypt hanya memakai 72 byte pertama (dan menolak lebih dari itu).
	maxPasswordBytes = 72
)

// ValidatePassword: aturan password yang sama untuk registrasi dan reset (8 karakter sampai 72 byte).
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordRunes || len(password) > maxPasswordBytes {
		return apperror.ErrWeakPassword
	}
	return nil
}

type PasswordResetRepository interface {
	// FindForReset: user dengan email persis ini; apperror.ErrNotFound bila tidak ada.
	FindForReset(ctx context.Context, email string) (model.User, error)
	CountRecentTokens(ctx context.Context, userID int64, purpose string, since time.Time) (int, error)
	// CreateToken menyimpan token baru dan membatalkan token lama yang belum terpakai (purpose sama) dalam satu transaksi.
	CreateToken(ctx context.Context, userID int64, purpose, tokenHash string, expiresAt time.Time, requestIP string) error
	// ResetPassword menandai token terpakai, mengganti password, mengisi password_changed_at, dan membatalkan
	// token lain milik user, semuanya atomik. apperror.ErrInvalidResetToken bila token tidak ada, kedaluwarsa,
	// atau sudah dipakai.
	ResetPassword(ctx context.Context, tokenHash, passwordHash string) error
}

// ResetMailer: pengirim email (mailclient.Client).
type ResetMailer interface {
	Send(ctx context.Context, to, template string, data map[string]any) error
}

type PasswordResetService struct {
	repo      PasswordResetRepository
	mailer    ResetMailer
	publicURL string
	now       func() time.Time
	log       *slog.Logger
}

func NewPasswordResetService(repo PasswordResetRepository, mailer ResetMailer, publicAppURL string) *PasswordResetService {
	return &PasswordResetService{
		repo:      repo,
		mailer:    mailer,
		publicURL: strings.TrimRight(publicAppURL, "/"),
		now:       time.Now,
		log:       slog.Default(),
	}
}

// RequestReset membuat token dan mengirim tautan ke email akun. Akun yang tidak ada, atau yang sudah mencapai
// batas per jam, dikembalikan sebagai nil supaya pemanggil tidak bisa membedakannya (tidak ada enumerasi).
// Token mentah hanya ada di email; yang disimpan hanya hash-nya.
func (s *PasswordResetService) RequestReset(ctx context.Context, email, requestIP string) error {
	user, err := s.repo.FindForReset(ctx, strings.TrimSpace(email))
	if err == apperror.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	now := s.now()
	recent, err := s.repo.CountRecentTokens(ctx, user.ID, PurposePasswordReset, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if recent >= resetMaxPerHour {
		s.log.Warn("password reset rate limited", "user_id", user.ID)
		return nil
	}

	raw, err := newActionToken()
	if err != nil {
		return err
	}
	if err := s.repo.CreateToken(ctx, user.ID, PurposePasswordReset, hashActionToken(raw), now.Add(resetTokenTTL), requestIP); err != nil {
		return err
	}
	return s.mailer.Send(ctx, user.Email, "password-reset", map[string]any{
		"Name":    user.Username,
		"Link":    s.publicURL + "/reset-password?token=" + raw,
		"Minutes": int(resetTokenTTL / time.Minute),
	})
}

// ResetPassword memasang password baru bila token sah. Password diperiksa sebelum token disentuh, jadi password
// yang terlalu pendek tidak menghabiskan token.
func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return apperror.ErrInvalidResetToken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.ResetPassword(ctx, hashActionToken(token), string(hash))
}

// newActionToken: 32 byte acak dari crypto/rand, base64url tanpa padding (43 karakter).
func newActionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashActionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
