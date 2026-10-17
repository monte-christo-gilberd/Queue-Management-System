package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/service"
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const specJWTSecret = "spec-secret"

func newSpecUserService() (*service.UserService, *memUserRepo) {
	repo := newMemUserRepo()
	return service.NewUserService(repo, specJWTSecret), repo
}

func parseSub(t *testing.T, token string) string {
	t.Helper()
	parsed, err := jwt.Parse(token, func(*jwt.Token) (interface{}, error) {
		return []byte(specJWTSecret), nil
	})
	require.NoError(t, err)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	sub, ok := claims["sub"].(string)
	require.True(t, ok)
	return sub
}

func TestSpec_Register_NewAccountIsLoggedInWithToken(t *testing.T) {
	// TC-01-06: registrasi langsung menghasilkan sesi (token) yang berisi user baru.
	svc, repo := newSpecUserService()

	token, err := svc.Register(context.Background(), "owner@example.com", "owner", "rahasia123")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Len(t, repo.users, 1)
	assert.Equal(t, "1", parseSub(t, token))
}

func TestSpec_Register_DuplicateEmailIsRejectedWithoutToken(t *testing.T) {
	// TC-01-07: email sudah terdaftar -> gagal, tidak ada token.
	svc, _ := newSpecUserService()
	ctx := context.Background()
	_, err := svc.Register(ctx, "dup@example.com", "first", "rahasia123")
	require.NoError(t, err)

	token, err := svc.Register(ctx, "dup@example.com", "second", "lainnya456")
	assert.ErrorIs(t, err, apperror.ErrDuplicate)
	assert.Empty(t, token)
}

func TestSpec_Register_PasswordIsStoredHashed(t *testing.T) {
	// Data model 4.17: kolom password_hash; kata sandi tidak disimpan apa adanya.
	svc, repo := newSpecUserService()
	_, err := svc.Register(context.Background(), "hash@example.com", "hasher", "rahasia123")
	require.NoError(t, err)

	require.Len(t, repo.users, 1)
	stored := repo.users[0].PasswordHash
	require.True(t, stored.Valid)
	assert.NotEqual(t, "rahasia123", stored.String)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.String), []byte("rahasia123")))
}

func TestSpec_Login_EmailOrUsernameWithCorrectPasswordGetsToken(t *testing.T) {
	// TC-01-01 / TC-01-02: login dengan email atau username.
	svc, _ := newSpecUserService()
	ctx := context.Background()
	_, err := svc.Register(ctx, "login@example.com", "loginuser", "rahasia123")
	require.NoError(t, err)

	for _, id := range []string{"login@example.com", "loginuser"} {
		token, err := svc.Login(ctx, id, "rahasia123")
		require.NoError(t, err, id)
		assert.Equal(t, "1", parseSub(t, token), id)
	}
}

func TestSpec_Login_WrongPasswordAndUnknownAccountLookIdentical(t *testing.T) {
	// TC-01-03 / TC-01-04: pesan gagal sama untuk password salah dan akun tidak terdaftar,
	// dan tidak ada token yang terbit.
	svc, _ := newSpecUserService()
	ctx := context.Background()
	_, err := svc.Register(ctx, "known@example.com", "known", "rahasia123")
	require.NoError(t, err)

	wrongToken, wrongErr := svc.Login(ctx, "known@example.com", "salah")
	unknownToken, unknownErr := svc.Login(ctx, "tidak@ada.com", "salah")

	assert.Empty(t, wrongToken)
	assert.Empty(t, unknownToken)
	require.Error(t, wrongErr)
	require.Error(t, unknownErr)
	assert.Equal(t, wrongErr.Error(), unknownErr.Error())
}

func TestSpec_Profile_ReportsOwnerFlagForOwnerOnly(t *testing.T) {
	// 4.18 / 4.1: profil menandai apakah user adalah owner (punya langganan/baris owner).
	svc, repo := newSpecUserService()
	ctx := context.Background()
	_, err := svc.Register(ctx, "pro@example.com", "pro", "rahasia123")
	require.NoError(t, err)
	_, err = svc.Register(ctx, "cust@example.com", "cust", "rahasia123")
	require.NoError(t, err)
	repo.owners[1] = true

	_, isOwner, err := svc.Profile(ctx, 1)
	require.NoError(t, err)
	assert.True(t, isOwner)

	_, isOwner, err = svc.Profile(ctx, 2)
	require.NoError(t, err)
	assert.False(t, isOwner)
}
