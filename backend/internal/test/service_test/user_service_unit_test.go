package service_test

import (
	"context"
	"qms/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSpecUserService membuat UserService dengan repository di memori. Registrasi dan login ada di auth-service,
// jadi akun dibuat langsung lewat repository.
func newSpecUserService() (*service.UserService, *memUserRepo) {
	repo := newMemUserRepo()
	return service.NewUserService(repo), repo
}

func TestSpec_Profile_ReportsOwnerFlagForOwnerOnly(t *testing.T) {
	// 4.18 / 4.1: profil menandai apakah user adalah owner (punya langganan/baris owner).
	svc, repo := newSpecUserService()
	ctx := context.Background()
	_, err := repo.Create(ctx, "pro@example.com", "pro", "hash")
	require.NoError(t, err)
	_, err = repo.Create(ctx, "cust@example.com", "cust", "hash")
	require.NoError(t, err)
	repo.owners[1] = true

	p, err := svc.Profile(ctx, 1)
	require.NoError(t, err)
	assert.True(t, p.IsOwner)

	p, err = svc.Profile(ctx, 2)
	require.NoError(t, err)
	assert.False(t, p.IsOwner)
}

func TestProfile_EmailVerifiedFollowsVerificationSwitch(t *testing.T) {
	// Verifikasi mati: semua akun dilaporkan terverifikasi (klien tidak menampilkan banner).
	// Verifikasi menyala: hanya akun dengan email_verified_at yang terverifikasi.
	repo := newMemUserRepo()
	_, err := repo.Create(context.Background(), "a@example.com", "a", "hash")
	require.NoError(t, err)

	off, err := service.NewUserService(repo).Profile(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, off.EmailVerified)

	on, err := service.NewUserService(repo).WithEmailVerification(true).Profile(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, on.EmailVerified)
}
