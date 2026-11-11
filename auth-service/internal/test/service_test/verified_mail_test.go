package service_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/service"
	"qms/auth-service/internal/test/testutil"
)

// verifyToken membuat token verifikasi lewat SendVerification lalu mengosongkan rekaman email agar hanya email
// konfirmasi yang terhitung pada pengecekan berikutnya.
func verifyToken(t *testing.T, env *testutil.Env, id int64) string {
	t.Helper()
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/verify-email")
	env.Mailer.Notify = make(chan struct{}, 4)
	return raw
}

// waitMail menunggu satu pengiriman di latar belakang atau menggagalkan tes bila tidak datang.
func waitMail(t *testing.T, env *testutil.Env) {
	t.Helper()
	select {
	case <-env.Mailer.Notify:
	case <-time.After(2 * time.Second):
		t.Fatal("email konfirmasi tidak terkirim")
	}
}

// assertNoMoreMail memastikan tidak ada pengiriman tambahan dalam jeda singkat.
func assertNoMoreMail(t *testing.T, env *testutil.Env, want int) {
	t.Helper()
	time.Sleep(150 * time.Millisecond)
	assert.Len(t, env.Mailer.Sent(), want)
}

func TestVerifyEmail_SendsOneConfirmationMail(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	raw := verifyToken(t, env, id)

	require.NoError(t, env.Svc.VerifyEmail(ctx, raw))
	waitMail(t, env)

	sent := env.Mailer.Sent()
	require.Len(t, sent, 2)
	assert.Equal(t, "email-verified", sent[1].Template)
	assert.Equal(t, "budi@mail.com", sent[1].To)
	assert.Equal(t, map[string]any{"Name": "Budi", "Link": "https://app.example/"}, sent[1].Data)
	// token sudah terpakai: pemakaian ulang gagal dan tidak menambah email
	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, raw), apperror.ErrInvalidResetToken)
	assertNoMoreMail(t, env, 2)
}

func TestVerifyEmail_MailFailureDoesNotChangeResult(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	raw := verifyToken(t, env, id)
	env.Mailer.Err = errors.New("smtp down")

	require.NoError(t, env.Svc.VerifyEmail(ctx, raw))
	waitMail(t, env)

	assert.True(t, env.Store.User(id).EmailVerifiedAt.Valid)
}

func TestVerifyEmail_InvalidTokenSendsNoMail(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	verifyToken(t, env, id)

	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, "salah"), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, ""), apperror.ErrInvalidResetToken)

	assertNoMoreMail(t, env, 1)
}

func TestVerifyEmail_WithoutMailerSucceedsSilently(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	raw := verifyToken(t, env, id)
	issuer := service.NewTokenIssuer(testutil.Key, "test-kid")
	noMail := service.NewService(env.Store, env.Store, issuer, nil, service.Options{PublicAppURL: "https://app.example/"})

	require.NoError(t, noMail.VerifyEmail(ctx, raw))

	assert.True(t, env.Store.User(id).EmailVerifiedAt.Valid)
	assertNoMoreMail(t, env, 1)
}

func TestVerifyEmail_AlreadyVerifiedAccountSendsNoMail(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", true)
	sum := sha256.Sum256([]byte("raw-token"))
	require.NoError(t, env.Store.CreateToken(ctx, id, service.PurposeEmailVerify, hex.EncodeToString(sum[:]), time.Now().Add(time.Hour), ""))

	require.NoError(t, env.Svc.VerifyEmail(ctx, "raw-token"))

	assertNoMoreMail(t, env, 0)
}
