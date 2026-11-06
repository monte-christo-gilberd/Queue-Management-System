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

func NewEmailVerificationService(
	repo EmailVerificationRepository,
	mailer ResetMailer,
	publicAppURL string,
) *EmailVerificationService {
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

	user, err := s.repo.FindForVerification(
		ctx,
		userID,
	)

	if err != nil {
		return err
	}
	if user.EmailVerifiedAt.Valid {
		return nil
	}

	now := s.now()
	recent, err := s.repo.CountRecentTokens(
		ctx,
		userID,
		PurposeEmailVerify,
		now.Add(-time.Hour),
	)

	if err != nil {
		return err
	}
	if recent >= verifyMaxPerHour {

		s.log.Warn(
			"verification email rate limited",
			"user_id",
			userID,
		)

		return apperror.ErrTooManyVerifyEmails
	}

	raw, err := newActionToken()
	if err != nil {
		return err
	}
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
	token = strings.TrimSpace(token)

	if token == "" {
		return apperror.ErrInvalidResetToken
	}
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
	
	verified, err := s.repo.IsVerified(
		ctx,
		userID,
	)

	if err != nil {
		return err
	}
	if !verified {
		return apperror.ErrEmailNotVerified
	}
	return nil
}
