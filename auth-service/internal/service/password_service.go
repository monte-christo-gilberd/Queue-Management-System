package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
)

const (
	// PurposePasswordReset dan PurposeEmailVerify: nilai kolom user_action_tokens.purpose.
	PurposePasswordReset = "password_reset"
	PurposeEmailVerify   = "email_verify"

	resetTokenTTL = 30 * time.Minute
	// Batas per akun per jam, di atas batas per IP di nginx.
	resetMaxPerHour = 3
)

// RequestReset membuat token dan mengirim tautan ke email akun. Akun yang tidak ada, atau yang sudah mencapai
// batas per jam, dikembalikan sebagai nil supaya pemanggil tidak bisa membedakannya (tidak ada enumerasi).
func (s *Service) RequestReset(
	ctx context.Context,
	email,
	requestIP string,
) error {
	// tanpa pengirim email, reset tidak mungkin; rute juga tidak dipasang
	if s.mailer == nil {
		// Email mati: reset tidak bisa dijalankan.
		return errors.New("mailer not configured")
	}
	// cari akun berdasarkan email; email tidak terdaftar tidak dianggap galat (tidak ada enumerasi)
	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	// Email tidak terdaftar: diam-diam anggap berhasil agar tidak membocorkan keberadaan akun.
	if errors.Is(err, apperror.ErrNotFound) {
		// Kembalikan nil seolah email terkirim.
		return nil
	}
	// Gangguan database saat mencari akun.
	if err != nil {
		// Kembalikan error database.
		return err
	}
	// Ambil waktu sekali agar hitungan batas dan masa berlaku token konsisten.
	now := s.now()
	// hitung token reset yang sudah dibuat akun ini dalam satu jam terakhir
	recent, err := s.tokens.CountRecentTokens(ctx, user.ID, PurposePasswordReset, now.Add(-time.Hour))
	// Gagal menghitung token terbaru.
	if err != nil {
		// Kembalikan error database.
		return err
	}
	// batas per jam tercapai: catat lalu kembalikan nil tanpa mengirim email
	if recent >= resetMaxPerHour {
		// Catat pembatasan untuk pemantauan; pemanggil tetap mendapat respons sukses.
		s.log.Warn("password reset rate limited", "user_id", user.ID)
		// Jangan kirim email dan jangan beri tahu pemanggil (tidak ada enumerasi).
		return nil
	}
	// token mentah acak yang hanya ada di email
	raw, err := newActionToken()
	// Gagal membuat token acak.
	if err != nil {
		// Kembalikan error pembuatan token.
		return err
	}
	// simpan hash token (bukan token mentah); token lama yang belum dipakai dibatalkan
	if err := s.tokens.CreateToken(ctx, user.ID, PurposePasswordReset, hashActionToken(raw), now.Add(resetTokenTTL), requestIP); err != nil {
		// Gagal menyimpan token: kembalikan error.
		return err
	}
	// kirim tautan reset yang memuat token mentah dan lama berlakunya dalam menit
	return s.mailer.Send(ctx, user.Email, "password-reset", map[string]any{
		"Name":    user.Username,
		"Link":    s.publicURL + "/reset-password?token=" + raw,
		"Minutes": int(resetTokenTTL / time.Minute),
	})
}

// ResetPassword memasang password baru bila token sah. Password diperiksa sebelum token disentuh.
func (s *Service) ResetPassword(
	ctx context.Context,
	token,
	newPassword string,
) error {
	// password lemah ditolak sebelum token disentuh, jadi tidak menghabiskan token
	if err := ValidatePassword(newPassword); err != nil {
		// Password baru tidak memenuhi aturan: kembalikan error.
		return err
	}
	// buang spasi di ujung token; token kosong tidak mungkin sah
	token = strings.TrimSpace(token)
	// Token kosong tidak mungkin sah: tolak.
	if token == "" {
		// Gunakan error yang sama dengan token salah supaya tidak membedakan sebabnya.
		return apperror.ErrInvalidResetToken
	}
	// hash password baru, lalu tandai token terpakai dan simpan password secara atomik di store
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	// Gagal membuat hash password.
	if err != nil {
		// Bungkus error dengan konteks untuk log.
		return fmt.Errorf("hash password: %w", err)
	}
	// Simpan password baru dan tandai token terpakai; token tidak sah menghasilkan ErrInvalidResetToken.
	return s.tokens.ResetPassword(ctx, hashActionToken(token), string(hash))
}

// newActionToken: 32 byte acak dari crypto/rand, base64url tanpa padding (43 karakter).
func newActionToken() (string, error) {
	// Siapkan 32 byte untuk token acak.
	b := make([]byte, 32)
	// isi dengan byte acak dari crypto/rand; gagal berarti sumber keacakan bermasalah
	if _, err := rand.Read(b); err != nil {
		// Bungkus error agar jelas berasal dari pembuatan token.
		return "", fmt.Errorf("generate token: %w", err)
	}
	// Kodekan ke base64url tanpa padding agar aman dipakai di URL.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashActionToken: SHA-256 token mentah (hex). Hanya hash ini yang disimpan di database.
func hashActionToken(raw string) string {
	// hitung SHA-256 lalu ubah menjadi string hex
	sum := sha256.Sum256([]byte(raw))
	// Kembalikan hash dalam bentuk hex.
	return hex.EncodeToString(sum[:])
}
