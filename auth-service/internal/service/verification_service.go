package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"qms/auth-service/internal/apperror"
)

const (
	verifyTokenTTL   = 24 * time.Hour
	verifyMaxPerHour = 3

	// verificationSendTimeout membatasi pengiriman email verifikasi di latar belakang saat registrasi.
	verificationSendTimeout = 45 * time.Second
)

// sendVerificationInBackground mengirim email verifikasi tanpa menahan respons registrasi.
func (s *Service) sendVerificationInBackground(userID int64) {
	// verifikasi mati: tidak ada email yang dikirim
	if !s.verifyOn {
		// Verifikasi mati: tidak ada yang dikirim.
		return
	}
	// jalankan di goroutine agar registrasi tidak menunggu SMTP yang lambat
	go func() {
		// context sendiri dengan batas waktu, tidak mengikuti request yang sudah selesai
		ctx, cancel := context.WithTimeout(context.Background(), verificationSendTimeout)
		defer cancel()
		// gagal kirim hanya dicatat (tanpa alamat email); user bisa kirim ulang dari Profil
		if err := s.SendVerification(ctx, userID, ""); err != nil {
			s.log.Error("send verification email after register failed", "user_id", userID, "err", err)
		}
	}()
}

// SendVerification membuat token baru dan mengirim tautan verifikasi. Akun yang sudah terverifikasi: tidak
// mengirim apa pun. Batas per jam habis: apperror.ErrTooManyVerifyEmails.
func (s *Service) SendVerification(
	ctx context.Context,
	userID int64,
	requestIP string,
) error {
	// tanpa pengirim email, tidak ada yang bisa dikirim
	if s.mailer == nil {
		// Email mati: pengiriman tidak bisa dilakukan.
		return errors.New("mailer not configured")
	}
	// ambil data user (email, username, status verifikasi)
	user, err := s.users.FindForVerification(ctx, userID)
	// Gagal membaca data user.
	if err != nil {
		// Kembalikan error (termasuk ErrNotFound bila user tidak ada).
		return err
	}
	// sudah terverifikasi: tidak ada yang perlu dikirim
	if user.EmailVerifiedAt.Valid {
		// Tidak perlu mengirim ulang email ke akun yang sudah terverifikasi.
		return nil
	}
	// Ambil waktu sekali agar hitungan batas dan masa berlaku token konsisten.
	now := s.now()
	// hitung token verifikasi yang sudah dibuat akun ini dalam satu jam terakhir
	recent, err := s.tokens.CountRecentTokens(ctx, userID, PurposeEmailVerify, now.Add(-time.Hour))
	// Gagal menghitung token terbaru.
	if err != nil {
		// Kembalikan error database.
		return err
	}
	// batas per jam tercapai: tolak dengan ErrTooManyVerifyEmails
	if recent >= verifyMaxPerHour {
		// Catat pembatasan untuk pemantauan; mencegah penyalahgunaan pengiriman email.
		s.log.Warn("verification email rate limited", "user_id", userID)
		// Tolak dengan error khusus agar klien menampilkan pesan batas.
		return apperror.ErrTooManyVerifyEmails
	}
	// token mentah acak yang hanya ada di email
	raw, err := newActionToken()
	// Gagal membuat token acak.
	if err != nil {
		// Kembalikan error pembuatan token.
		return err
	}
	// simpan hash token; token lama yang belum dipakai dibatalkan dalam satu transaksi
	if err := s.tokens.CreateToken(ctx, userID, PurposeEmailVerify, hashActionToken(raw), now.Add(verifyTokenTTL), requestIP); err != nil {
		// Gagal menyimpan token: kembalikan error.
		return err
	}
	// kirim tautan verifikasi yang memuat token mentah dan lama berlakunya dalam jam
	return s.mailer.Send(ctx, user.Email, "email-verify", map[string]any{
		"Name":  user.Username,
		"Link":  s.publicURL + "/verify-email?token=" + raw,
		"Hours": int(verifyTokenTTL / time.Hour),
	})
}

// VerifyEmail menandai email terverifikasi bila token sah; token kosong atau tidak sah: ErrInvalidResetToken.
func (s *Service) VerifyEmail(
	ctx context.Context,
	token string,
) error {
	// buang spasi di ujung token; token kosong tidak mungkin sah
	token = strings.TrimSpace(token)
	// Token kosong tidak mungkin sah: tolak.
	if token == "" {
		// Gunakan error yang sama dengan token salah.
		return apperror.ErrInvalidResetToken
	}
	// cocokkan hash token di database, tandai email terverifikasi dan token terpakai
	return s.tokens.VerifyEmail(ctx, hashActionToken(token))
}
