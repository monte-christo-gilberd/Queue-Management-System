package service

import (
	"context"
	"log/slog"
	"qms/internal/apperror"
	"qms/internal/model"
	"strings"
	"time"
)

const (
	// PurposeEmailVerify: nilai kolom user_action_tokens.purpose untuk token verifikasi email.
	PurposeEmailVerify = "email_verify"

	verifyTokenTTL = 24 * time.Hour
	// Batas per akun per jam, di atas batas rute di nginx: satu akun tidak bisa membanjiri kotak masuk sendiri
	// (atau alamat yang salah ketik) dengan email verifikasi.
	verifyMaxPerHour = 3
)

// EmailVerificationRepository: penyimpanan token verifikasi email dan status verifikasi user. Dipenuhi
// repository (email_verification_repo.go).
type EmailVerificationRepository interface {
	// FindForVerification: id, email, username, dan status verifikasi; apperror.ErrNotFound bila user tidak ada.
	FindForVerification(
		ctx context.Context,
		userID int64,
	) (model.User, error)
	// IsVerified: true bila email sudah diverifikasi.
	IsVerified(
		ctx context.Context,
		userID int64,
	) (bool, error)
	CountRecentTokens(
		ctx context.Context,
		userID int64,
		purpose string,
		since time.Time,
	) (int, error)
	// CreateToken menyimpan token baru dan membatalkan token lama yang belum terpakai (purpose sama) dalam satu transaksi.
	CreateToken(
		ctx context.Context,
		userID int64,
		purpose,
		tokenHash string,
		expiresAt time.Time,
		requestIP string,
	) error
	// VerifyEmail menandai token terpakai dan mengisi email_verified_at secara atomik.
	// apperror.ErrInvalidResetToken bila token tidak ada, kedaluwarsa, atau sudah dipakai.
	VerifyEmail(
		ctx context.Context,
		tokenHash string,
	) error
}

// VerificationGuard memeriksa bahwa email user sudah diverifikasi (apperror.ErrEmailNotVerified bila belum).
// Dipenuhi *EmailVerificationService dan dipasang ke QueueService dan TicketService; guard yang tidak dipasang
// (verifikasi mati) berarti tidak ada pemeriksaan.
type VerificationGuard interface {
	RequireVerified(
		ctx context.Context,
		userID int64,
	) error
}

// EmailVerificationService: verifikasi email lewat tautan sekali pakai. Token mentah hanya ada di email; yang
// disimpan hanya hash-nya (token dan hash dibuat oleh helper yang sama dengan reset password).
//
// Layanan ini hanya dibuat bila verifikasi dinyalakan (EMAIL_VERIFICATION_REQUIRED), jadi memilikinya berarti
// verifikasi berlaku: RequireVerified menolak akun yang belum terverifikasi.
type EmailVerificationService struct {
	repo      EmailVerificationRepository
	mailer    ResetMailer
	publicURL string
	now       func() time.Time
	log       *slog.Logger
}

// NewEmailVerificationService membuat layanan verifikasi email. publicAppURL dipakai untuk membuat tautan di
// email; garis miring di akhirnya dibuang.
func NewEmailVerificationService(
	repo EmailVerificationRepository,
	mailer ResetMailer,
	publicAppURL string,
) *EmailVerificationService {
	// Rakit layanan: garis miring di akhir URL dibuang, waktu memakai time.Now, dan logger default.
	return &EmailVerificationService{
		repo:      repo,
		mailer:    mailer,
		publicURL: strings.TrimRight(publicAppURL, "/"),
		now:       time.Now,
		log:       slog.Default(),
	}
}

// SendVerification membuat token baru (token lama dibatalkan) dan mengirim tautan ke email akun.
// Akun yang sudah terverifikasi: tidak mengirim apa pun dan tidak error. Sudah mencapai batas per jam:
// apperror.ErrTooManyVerifyEmails. requestIP boleh kosong (mis. saat registrasi).
func (s *EmailVerificationService) SendVerification(
	ctx context.Context,
	userID int64,
	requestIP string,
) error {

	// Ambil data user (id, email, username, status verifikasi) dari database.
	user, err := s.repo.FindForVerification(
		ctx,
		userID,
	)

	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Email sudah terverifikasi: tidak perlu mengirim apa pun, cukup kembalikan nil.
	if user.EmailVerifiedAt.Valid {
		return nil
	}

	now := s.now()
	// Hitung token verifikasi yang sudah dibuat akun ini dalam satu jam terakhir.
	recent, err := s.repo.CountRecentTokens(
		ctx,
		userID,
		PurposeEmailVerify,
		now.Add(-time.Hour),
	)

	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Sudah mencapai batas per jam: catat peringatan lalu tolak dengan ErrTooManyVerifyEmails.
	if recent >= verifyMaxPerHour {

		s.log.Warn(
			"verification email rate limited",
			"user_id",
			userID,
		)

		return apperror.ErrTooManyVerifyEmails
	}

	// Buat token mentah acak yang akan dikirim lewat email.
	raw, err := newActionToken()
	// gagal membuat token: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Simpan hash token beserta waktu kedaluwarsa; token lama yang belum dipakai dibatalkan dalam satu transaksi.
	if err := s.repo.CreateToken(
		ctx,
		userID,
		PurposeEmailVerify,
		hashActionToken(raw),
		now.Add(verifyTokenTTL),
		requestIP,
	); err != nil {
		return err
	}
	// Kirim email berisi tautan verifikasi yang memuat token mentah dan lama berlakunya dalam jam.
	return s.mailer.Send(
		ctx,
		user.Email,
		"email-verify",
		map[string]any{
			"Name":  user.Username,
			"Link":  s.publicURL + "/verify-email?token=" + raw,
			"Hours": int(verifyTokenTTL / time.Hour),
		})
}

// Verify menandai email terverifikasi bila token sah. Token kosong, tidak ada, kedaluwarsa, atau sudah dipakai:
// apperror.ErrInvalidResetToken (satu galat untuk semuanya, sama dengan reset password).
func (s *EmailVerificationService) Verify(
	ctx context.Context,
	token string,
) error {
	// Buang spasi di ujung token agar tidak salah cocok.
	token = strings.TrimSpace(token)

	// Token kosong tidak mungkin sah: tolak sebagai token tidak valid.
	if token == "" {
		return apperror.ErrInvalidResetToken
	}
	// Cocokkan hash token dengan database, lalu tandai email terverifikasi dan token terpakai.
	return s.repo.VerifyEmail(
		ctx,
		hashActionToken(token),
	)
}

// RequireVerified mengembalikan apperror.ErrEmailNotVerified bila email user belum diverifikasi.
// Dibaca dari database pada tiap panggilan (bukan dari klaim JWT), supaya verifikasi berlaku tanpa login ulang.
func (s *EmailVerificationService) RequireVerified(
	ctx context.Context,
	userID int64,
) error {

	// Baca status verifikasi email user dari database.
	verified, err := s.repo.IsVerified(
		ctx,
		userID,
	)

	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return err
	}
	// Email belum diverifikasi: tolak dengan ErrEmailNotVerified.
	if !verified {
		return apperror.ErrEmailNotVerified
	}
	return nil
}
