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
	// Hitung jumlah karakter (rune) dan ukuran byte; password di luar batas ditolak.
	if utf8.RuneCountInString(password) < minPasswordRunes || len(password) > maxPasswordBytes {
		return apperror.ErrWeakPassword
	}
	return nil
}

// PasswordResetRepository: penyimpanan token reset dan penggantian password. Dipenuhi repository (password_reset_repo.go).
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

// PasswordResetService: alur reset password lewat tautan sekali pakai. Token berlaku 30 menit, dan tiap akun
// hanya bisa meminta 3 token per jam.
type PasswordResetService struct {
	repo      PasswordResetRepository
	mailer    ResetMailer
	publicURL string
	now       func() time.Time
	log       *slog.Logger
}

// NewPasswordResetService membuat layanan reset password. publicAppURL dipakai untuk membuat tautan di email;
// garis miring di akhirnya dibuang supaya tidak ada "//" di tautan.
func NewPasswordResetService(repo PasswordResetRepository, mailer ResetMailer, publicAppURL string) *PasswordResetService {
	// Rakit layanan: garis miring di akhir URL dibuang, waktu memakai time.Now, dan logger default.
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
	// Cari user berdasarkan email (spasi di ujung dibuang); email tidak terdaftar tidak dianggap error.
	user, err := s.repo.FindForReset(ctx, strings.TrimSpace(email))
	// Email tidak terdaftar: kembalikan nil agar pemanggil tidak bisa menebak email mana yang terdaftar.
	if err == apperror.ErrNotFound {
		return nil
	}
	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}

	now := s.now()
	// Hitung token reset yang sudah dibuat akun ini dalam satu jam terakhir.
	recent, err := s.repo.CountRecentTokens(ctx, user.ID, PurposePasswordReset, now.Add(-time.Hour))
	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Sudah mencapai batas per jam: catat peringatan, lalu kembalikan nil tanpa mengirim email.
	if recent >= resetMaxPerHour {
		s.log.Warn("password reset rate limited", "user_id", user.ID)
		return nil
	}

	// Buat token mentah acak yang akan dikirim lewat email.
	raw, err := newActionToken()
	// gagal membuat token: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Simpan hash token (bukan token mentah) beserta waktu kedaluwarsa; token lama yang belum dipakai dibatalkan.
	if err := s.repo.CreateToken(ctx, user.ID, PurposePasswordReset, hashActionToken(raw), now.Add(resetTokenTTL), requestIP); err != nil {
		return err
	}
	// Kirim email berisi tautan reset yang memuat token mentah dan lama berlakunya dalam menit.
	return s.mailer.Send(ctx, user.Email, "password-reset", map[string]any{
		"Name":    user.Username,
		"Link":    s.publicURL + "/reset-password?token=" + raw,
		"Minutes": int(resetTokenTTL / time.Minute),
	})
}

// ResetPassword memasang password baru bila token sah. Password diperiksa sebelum token disentuh, jadi password
// yang terlalu pendek tidak menghabiskan token.
func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Periksa aturan password lebih dulu; password lemah ditolak sebelum token disentuh.
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	// Buang spasi di ujung token agar tidak salah cocok.
	token = strings.TrimSpace(token)
	// Token kosong tidak mungkin sah: tolak sebagai token tidak valid.
	if token == "" {
		return apperror.ErrInvalidResetToken
	}
	// Hash password baru dengan bcrypt (cost 10); gagal berarti password tidak bisa diproses.
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	// gagal hashing: kembalikan error ke pemanggil.
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	// Tandai token terpakai dan simpan password baru (hash) secara atomik di repository.
	return s.repo.ResetPassword(ctx, hashActionToken(token), string(hash))
}

// newActionToken: 32 byte acak dari crypto/rand, base64url tanpa padding (43 karakter).
func newActionToken() (string, error) {
	b := make([]byte, 32)
	// Isi buffer dengan byte acak dari crypto/rand; gagal berarti sumber keacakan bermasalah.
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	// Ubah byte acak menjadi string base64url tanpa padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashActionToken: SHA-256 dari token mentah dalam bentuk hex. Yang disimpan di database hanya hash ini,
// jadi kebocoran tabel tidak membuat token yang masih berlaku bisa dipakai.
func hashActionToken(raw string) string {
	// Hitung SHA-256 dari token mentah.
	sum := sha256.Sum256([]byte(raw))
	// Ubah hash byte menjadi string hex.
	return hex.EncodeToString(sum[:])
}
