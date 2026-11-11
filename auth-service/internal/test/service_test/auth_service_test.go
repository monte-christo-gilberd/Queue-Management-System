package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/service"
	"qms/auth-service/internal/test/testutil"
)

var ctx = context.Background()

// --- Registrasi ---

func TestRegister_NewAccountGetsRS256TokenForDirectEntry(t *testing.T) {
	// TC-01-06: registrasi berhasil -> token langsung diberikan (tanpa login ulang).
	env := testutil.NewService(t, false)

	token, err := env.Svc.Register(ctx, "owner@example.com", "owner", "rahasia123")

	require.NoError(t, err)
	id, err := env.Svc.Authenticate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.Equal(t, "owner@example.com", env.Store.User(1).Email)
}

func TestRegister_PasswordIsStoredHashedNeverPlain(t *testing.T) {
	env := testutil.NewService(t, false)

	_, err := env.Svc.Register(ctx, "hash@example.com", "hasher", "rahasia123")
	require.NoError(t, err)

	stored := env.Store.User(1).PasswordHash.String
	assert.NotContains(t, stored, "rahasia123")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte("rahasia123")))
}

func TestRegister_RejectsWeakPasswordWithoutCreatingAccount(t *testing.T) {
	env := testutil.NewService(t, false)

	for name, pw := range map[string]string{
		"pendek (7)":  "1234567",
		"kosong":      "",
		"73 byte":     strings.Repeat("a", 73),
		"7 rune unik": "ééééééé",
	} {
		token, err := env.Svc.Register(ctx, name+"@example.com", "u", pw)
		assert.ErrorIs(t, err, apperror.ErrWeakPassword, name)
		assert.Empty(t, token, name)
	}
	assert.Empty(t, env.Store.Users, "tidak ada akun yang dibuat")
}

func TestRegister_DuplicateEmailIsRejectedWithoutToken(t *testing.T) {
	env := testutil.NewService(t, false)
	_, err := env.Svc.Register(ctx, "dup@example.com", "first", "rahasia123")
	require.NoError(t, err)

	token, err := env.Svc.Register(ctx, "dup@example.com", "second", "lainnya456")

	assert.ErrorIs(t, err, apperror.ErrDuplicate)
	assert.Empty(t, token)
}

func TestRegister_SendsVerificationEmailInBackground(t *testing.T) {
	env := testutil.NewService(t, true)
	env.Mailer.Notify = make(chan struct{}, 1)

	token, err := env.Svc.Register(ctx, "baru@example.com", "baru", "rahasia123")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	select {
	case <-env.Mailer.Notify:
	case <-time.After(2 * time.Second):
		t.Fatal("email verifikasi tidak dikirim setelah registrasi")
	}
	sent := env.Mailer.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "baru@example.com", sent[0].To)
	assert.Equal(t, "email-verify", sent[0].Template)
}

func TestRegister_SucceedsEvenWhenVerificationEmailFails(t *testing.T) {
	env := testutil.NewService(t, true)
	env.Mailer.Err = errors.New("smtp down")
	env.Mailer.Notify = make(chan struct{}, 1)

	token, err := env.Svc.Register(ctx, "baru@example.com", "baru", "rahasia123")

	require.NoError(t, err)
	assert.NotEmpty(t, token, "kegagalan kirim email tidak menggagalkan registrasi")
	<-env.Mailer.Notify
}

func TestRegister_NoEmailWhenVerificationOffOrDuplicate(t *testing.T) {
	off := testutil.NewService(t, false)
	_, err := off.Svc.Register(ctx, "a@example.com", "a", "rahasia123")
	require.NoError(t, err)

	on := testutil.NewService(t, true)
	on.Mailer.Notify = make(chan struct{}, 4)
	_, err = on.Svc.Register(ctx, "dup@example.com", "first", "rahasia123")
	require.NoError(t, err)
	<-on.Mailer.Notify
	_, err = on.Svc.Register(ctx, "dup@example.com", "second", "lainnya456")
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, off.Mailer.Sent(), "verifikasi mati: tidak ada email")
	assert.Len(t, on.Mailer.Sent(), 1, "registrasi yang gagal tidak mengirim email verifikasi")
}

// --- Login ---

func registered(t *testing.T, env *testutil.Env, email, username, password string) {
	t.Helper()
	_, err := env.Svc.Register(ctx, email, username, password)
	require.NoError(t, err)
}

func TestLogin_EmailOrUsernameWithCorrectPasswordGetsToken(t *testing.T) {
	// TC-01-01 / TC-01-02
	env := testutil.NewService(t, false)
	registered(t, env, "login@example.com", "loginuser", "rahasia123")

	for _, id := range []string{"login@example.com", "loginuser"} {
		token, err := env.Svc.Login(ctx, id, "rahasia123")
		require.NoError(t, err, id)
		uid, err := env.Svc.Authenticate(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, int64(1), uid)
	}
}

func TestLogin_WrongPasswordAndUnknownAccountLookIdentical(t *testing.T) {
	// TC-01-03 / TC-01-04: tidak membocorkan akun mana yang terdaftar.
	env := testutil.NewService(t, false)
	registered(t, env, "known@example.com", "known", "rahasia123")

	wrongToken, wrongErr := env.Svc.Login(ctx, "known@example.com", "salah")
	unknownToken, unknownErr := env.Svc.Login(ctx, "tidak@ada.com", "salah")

	assert.ErrorIs(t, wrongErr, apperror.ErrNotFound)
	assert.ErrorIs(t, unknownErr, apperror.ErrNotFound)
	assert.Empty(t, wrongToken)
	assert.Empty(t, unknownToken)
}

func TestLogin_SharedUsernameMatchesByPassword(t *testing.T) {
	// Username tidak unik: yang cocok passwordnya yang masuk.
	env := testutil.NewService(t, false)
	registered(t, env, "a@example.com", "budi", "alpha-pass")
	registered(t, env, "b@example.com", "budi", "beta-pass")

	token, err := env.Svc.Login(ctx, "budi", "beta-pass")
	require.NoError(t, err)
	uid, err := env.Svc.Authenticate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, int64(2), uid)

	_, err = env.Svc.Login(ctx, "budi", "gamma-pass")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestLogin_SkipsGoogleOnlyAccounts(t *testing.T) {
	// Akun tanpa password (hanya Google) tidak bisa login dengan password, termasuk password kosong.
	env := testutil.NewService(t, false)
	env.Store.AddUser("g@example.com", "budi", "", true)

	token, err := env.Svc.Login(ctx, "budi", "")

	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Empty(t, token)
}

// --- Authenticate (dipakai middleware di auth-service) ---

func TestAuthenticate_PasswordChangedAfterIssueRevokesToken(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "u@example.com", "u", "rahasia123")
	token, err := env.Svc.Login(ctx, "u", "rahasia123")
	require.NoError(t, err)

	_, err = env.Svc.Authenticate(ctx, token)
	require.NoError(t, err, "sebelum password diganti, token sah")

	env.Store.SetPasswordChanged(1, time.Now().Add(2*time.Second))
	_, err = env.Svc.Authenticate(ctx, token)
	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}

func TestAuthenticate_PasswordChangedBeforeIssueIsAccepted(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "u@example.com", "u", "rahasia123")
	env.Store.SetPasswordChanged(1, time.Now().Add(-time.Hour))
	token, err := env.Svc.Login(ctx, "u", "rahasia123")
	require.NoError(t, err)

	_, err = env.Svc.Authenticate(ctx, token)

	assert.NoError(t, err)
}

func TestAuthenticate_RejectsBadTokensAndMapsDatabaseErrors(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "u@example.com", "u", "rahasia123")
	token, err := env.Svc.Login(ctx, "u", "rahasia123")
	require.NoError(t, err)

	_, err = env.Svc.Authenticate(ctx, "sampah")
	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)

	// sub bukan angka
	badSub, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "abc", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(testutil.Key)
	require.NoError(t, err)
	_, err = env.Svc.Authenticate(ctx, badSub)
	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)

	// user sudah tidak ada
	ghost, err := newIssuer().Issue(999)
	require.NoError(t, err)
	_, err = env.Svc.Authenticate(ctx, ghost)
	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)

	// gangguan database: bukan 401, supaya pemanggil membalas 500
	env.Store.Err = testutil.ErrBoom
	_, err = env.Svc.Authenticate(ctx, token)
	assert.ErrorIs(t, err, testutil.ErrBoom)
	assert.NotErrorIs(t, err, apperror.ErrUnauthenticated)
}

// --- Lupa dan reset password ---

// tokenFromMail mengambil token mentah dari tautan di email terkirim.
func tokenFromMail(t *testing.T, m testutil.Mail, path string) string {
	t.Helper()
	link, ok := m.Data["Link"].(string)
	require.True(t, ok)
	prefix := "https://app.example" + path + "?token="
	require.True(t, strings.HasPrefix(link, prefix), link)
	return strings.TrimPrefix(link, prefix)
}

func TestPasswordReset_RequestSendsLinkAndStoresOnlyHash(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")

	require.NoError(t, env.Svc.RequestReset(ctx, " budi@mail.com ", "1.2.3.4"))

	sent := env.Mailer.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "budi@mail.com", sent[0].To)
	assert.Equal(t, "password-reset", sent[0].Template)
	assert.Equal(t, "Budi", sent[0].Data["Name"])
	assert.Equal(t, 30, sent[0].Data["Minutes"])
	raw := tokenFromMail(t, sent[0], "/reset-password")
	assert.Len(t, raw, 43, "32 byte acak, base64url tanpa padding")
	for _, h := range env.Store.StoredHashes() {
		assert.NotEqual(t, raw, h, "token mentah tidak boleh disimpan")
		assert.Len(t, h, 64, "yang disimpan SHA-256 hex")
	}
}

func TestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	env := testutil.NewService(t, false)

	assert.NoError(t, env.Svc.RequestReset(ctx, "tidak@ada.com", ""))
	assert.Empty(t, env.Mailer.Sent(), "tidak ada email, dan tidak ada galat yang membedakan")
}

func TestPasswordReset_NewRequestRevokesOldToken(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	sent := env.Mailer.Sent()
	require.Len(t, sent, 2)

	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, tokenFromMail(t, sent[0], "/reset-password"), "password-baru"), apperror.ErrInvalidResetToken)
	assert.NoError(t, env.Svc.ResetPassword(ctx, tokenFromMail(t, sent[1], "/reset-password"), "password-baru"))
}

func TestPasswordReset_LimitThreePerHourPerAccount(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")

	for i := 0; i < 5; i++ {
		assert.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	}

	assert.Len(t, env.Mailer.Sent(), 3, "permintaan ke-4 dan ke-5 diam-diam diabaikan")
}

func TestPasswordReset_MailFailureIsReturnedForLogging(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")
	env.Mailer.Err = errors.New("smtp down")

	assert.Error(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
}

func TestPasswordReset_SetsPasswordOnceRecordsChangeAndRevokesOldSessions(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "lama-12345")
	oldToken, err := env.Svc.Login(ctx, "Budi", "lama-12345")
	require.NoError(t, err)
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/reset-password")

	time.Sleep(1100 * time.Millisecond) // iat berdetik: pastikan perubahan password berada di detik berikutnya
	require.NoError(t, env.Svc.ResetPassword(ctx, raw, "baru-12345"))

	_, err = env.Svc.Login(ctx, "Budi", "baru-12345")
	assert.NoError(t, err, "password baru bisa dipakai login")
	_, err = env.Svc.Login(ctx, "Budi", "lama-12345")
	assert.ErrorIs(t, err, apperror.ErrNotFound, "password lama tidak lagi berlaku")
	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, raw, "lagi-12345"), apperror.ErrInvalidResetToken, "token sekali pakai")
	_, err = env.Svc.Authenticate(ctx, oldToken)
	assert.ErrorIs(t, err, apperror.ErrUnauthenticated, "sesi lama dicabut")
}

func TestPasswordReset_ExpiredTokenRejected(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/reset-password")
	env.Store.ExpireTokens()

	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, raw, "baru-12345"), apperror.ErrInvalidResetToken)
}

func TestPasswordReset_WeakPasswordDoesNotConsumeToken(t *testing.T) {
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/reset-password")

	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, raw, "pendek"), apperror.ErrWeakPassword)
	assert.NoError(t, env.Svc.ResetPassword(ctx, raw, "cukup-panjang"), "token masih utuh")
}

func TestPasswordReset_UnknownOrEmptyTokenRejected(t *testing.T) {
	env := testutil.NewService(t, false)

	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, "", "baru-12345"), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, "   ", "baru-12345"), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, "tidak-ada", "baru-12345"), apperror.ErrInvalidResetToken)
}

func TestPasswordReset_EmailTokenCannotResetPassword(t *testing.T) {
	// Token verifikasi email tidak boleh dipakai untuk reset password (purpose berbeda).
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/verify-email")

	assert.ErrorIs(t, env.Svc.ResetPassword(ctx, raw, "baru-12345"), apperror.ErrInvalidResetToken)
}

func TestValidatePassword(t *testing.T) {
	assert.ErrorIs(t, service.ValidatePassword("1234567"), apperror.ErrWeakPassword)
	assert.NoError(t, service.ValidatePassword("12345678"))
	assert.NoError(t, service.ValidatePassword(strings.Repeat("a", 72)))
	assert.ErrorIs(t, service.ValidatePassword(strings.Repeat("a", 73)), apperror.ErrWeakPassword)
}

// --- Verifikasi email ---

func TestSendVerification_SendsLinkAndStoresOnlyHash(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)

	require.NoError(t, env.Svc.SendVerification(ctx, id, "1.2.3.4"))

	sent := env.Mailer.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "email-verify", sent[0].Template)
	assert.Equal(t, 24, sent[0].Data["Hours"])
	raw := tokenFromMail(t, sent[0], "/verify-email")
	for _, h := range env.Store.StoredHashes() {
		assert.NotEqual(t, raw, h)
	}
}

func TestSendVerification_AlreadyVerifiedSendsNothing(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("sari@mail.com", "Sari", "x", true)

	assert.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	assert.Empty(t, env.Mailer.Sent())
}

func TestSendVerification_UnknownUserIsNotFound(t *testing.T) {
	env := testutil.NewService(t, false)

	assert.ErrorIs(t, env.Svc.SendVerification(ctx, 999, ""), apperror.ErrNotFound)
}

func TestSendVerification_LimitedToThreePerHour(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)

	for i := 0; i < 3; i++ {
		require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	}

	assert.ErrorIs(t, env.Svc.SendVerification(ctx, id, ""), apperror.ErrTooManyVerifyEmails)
	assert.Len(t, env.Mailer.Sent(), 3)
}

func TestSendVerification_NewLinkInvalidatesOldOne(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	sent := env.Mailer.Sent()

	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, tokenFromMail(t, sent[0], "/verify-email")), apperror.ErrInvalidResetToken)
	assert.NoError(t, env.Svc.VerifyEmail(ctx, tokenFromMail(t, sent[1], "/verify-email")))
}

func TestSendVerification_MailFailureIsReturned(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	env.Mailer.Err = errors.New("smtp down")

	assert.Error(t, env.Svc.SendVerification(ctx, id, ""))
}

func TestVerifyEmail_MarksVerifiedAndTokenIsSingleUse(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/verify-email")

	require.NoError(t, env.Svc.VerifyEmail(ctx, " "+raw+" "))

	assert.True(t, env.Store.User(id).EmailVerifiedAt.Valid)
	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, raw), apperror.ErrInvalidResetToken)
}

func TestVerifyEmail_RejectsEmptyUnknownAndExpiredTokens(t *testing.T) {
	env := testutil.NewService(t, false)
	id := env.Store.AddUser("budi@mail.com", "Budi", "x", false)
	require.NoError(t, env.Svc.SendVerification(ctx, id, ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/verify-email")

	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, ""), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, "tidak-ada"), apperror.ErrInvalidResetToken)
	env.Store.ExpireTokens()
	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, raw), apperror.ErrInvalidResetToken)
	assert.False(t, env.Store.User(id).EmailVerifiedAt.Valid)
}

func TestVerifyEmail_ResetTokenCannotVerifyEmail(t *testing.T) {
	// Token reset password tidak boleh dipakai untuk memverifikasi email (purpose berbeda).
	env := testutil.NewService(t, false)
	registered(t, env, "budi@mail.com", "Budi", "rahasia123")
	require.NoError(t, env.Svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromMail(t, env.Mailer.Sent()[0], "/reset-password")

	assert.ErrorIs(t, env.Svc.VerifyEmail(ctx, raw), apperror.ErrInvalidResetToken)
}
