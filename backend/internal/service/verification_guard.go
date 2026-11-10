package service

import (
	"context"
	"qms/internal/apperror"
)

// VerificationStore: pembacaan status verifikasi email. Dipenuhi repository (verification_repo.go).
type VerificationStore interface {
	// IsVerified: true bila email sudah diverifikasi; apperror.ErrNotFound bila user tidak ada.
	IsVerified(
		ctx context.Context,
		userID int64,
	) (bool, error)
}

// VerificationGuard memeriksa bahwa email user sudah diverifikasi (apperror.ErrEmailNotVerified bila belum).
// Dipasang ke QueueService dan TicketService; guard yang tidak dipasang (verifikasi mati) berarti tidak ada
// pemeriksaan.
type VerificationGuard interface {
	RequireVerified(
		ctx context.Context,
		userID int64,
	) error
}

// EmailVerificationGuard memenuhi VerificationGuard dengan membaca users.email_verified_at langsung dari database.
// Pengiriman email dan penandaan terverifikasi dilakukan auth-service; backend hanya membaca hasilnya, jadi guard
// ini tetap bekerja saat auth-service mati.
type EmailVerificationGuard struct {
	// store: pembaca status verifikasi.
	store VerificationStore
}

// NewEmailVerificationGuard membuat guard yang membaca status dari store.
func NewEmailVerificationGuard(store VerificationStore) *EmailVerificationGuard {
	return &EmailVerificationGuard{store: store}
}

// RequireVerified mengembalikan apperror.ErrEmailNotVerified bila email user belum diverifikasi.
// Dibaca dari database pada tiap panggilan (bukan dari klaim JWT), supaya verifikasi berlaku tanpa login ulang.
func (g *EmailVerificationGuard) RequireVerified(
	ctx context.Context,
	userID int64,
) error {
	// baca status verifikasi email user dari database
	verified, err := g.store.IsVerified(ctx, userID)
	// gagal membaca database: kembalikan error ke pemanggil
	if err != nil {
		return err
	}
	// email belum diverifikasi: tolak dengan ErrEmailNotVerified
	if !verified {
		return apperror.ErrEmailNotVerified
	}
	return nil
}
