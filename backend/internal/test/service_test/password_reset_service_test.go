package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type fakeToken struct {
	userID    int64
	purpose   string
	expiresAt time.Time
	usedAt    *time.Time
	createdAt time.Time
}

// fakeResetRepo meniru semantik SQL: token sekali pakai, kedaluwarsa, token lama dibatalkan saat token baru dibuat.
type fakeResetRepo struct {
	mu        sync.Mutex
	users     []model.User
	tokens    map[string]*fakeToken // kunci: hash
	passwords map[int64]string
	changedAt map[int64]time.Time
}

func newFakeResetRepo(users ...model.User) *fakeResetRepo {
	return &fakeResetRepo{users: users, tokens: map[string]*fakeToken{}, passwords: map[int64]string{}, changedAt: map[int64]time.Time{}}
}

func (r *fakeResetRepo) FindForReset(_ context.Context, email string) (model.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

func (r *fakeResetRepo) CountRecentTokens(_ context.Context, userID int64, purpose string, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, t := range r.tokens {
		if t.userID == userID && t.purpose == purpose && t.createdAt.After(since) {
			n++
		}
	}
	return n, nil
}

func (r *fakeResetRepo) CreateToken(_ context.Context, userID int64, purpose, hash string, exp time.Time, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, t := range r.tokens {
		if t.userID == userID && t.purpose == purpose && t.usedAt == nil {
			t.usedAt = &now
		}
	}
	r.tokens[hash] = &fakeToken{userID: userID, purpose: purpose, expiresAt: exp, createdAt: now}
	return nil
}

func (r *fakeResetRepo) ResetPassword(_ context.Context, hash, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[hash]
	if !ok || t.usedAt != nil || !t.expiresAt.After(time.Now()) {
		return apperror.ErrInvalidResetToken
	}
	now := time.Now()
	t.usedAt = &now
	r.passwords[t.userID] = passwordHash
	r.changedAt[t.userID] = now
	return nil
}

type sentMail struct {
	to, template string
	data         map[string]any
}

type fakeResetMailer struct {
	sent []sentMail
	err  error
}

func (m *fakeResetMailer) Send(_ context.Context, to, template string, data map[string]any) error {
	m.sent = append(m.sent, sentMail{to, template, data})
	return m.err
}

func newResetSvc() (*service.PasswordResetService, *fakeResetRepo, *fakeResetMailer) {
	repo := newFakeResetRepo(model.User{ID: 7, Email: "budi@mail.com", Username: "Budi"})
	mailer := &fakeResetMailer{}
	return service.NewPasswordResetService(repo, mailer, "https://app.example/"), repo, mailer
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	const prefix = "https://app.example/reset-password?token="
	require.True(t, strings.HasPrefix(link, prefix), link)
	return strings.TrimPrefix(link, prefix)
}

func TestPasswordReset_RequestSendsLinkAndStoresOnlyHash(t *testing.T) {
	svc, repo, mailer := newResetSvc()

	require.NoError(t, svc.RequestReset(context.Background(), " budi@mail.com ", "1.2.3.4"))

	require.Len(t, mailer.sent, 1)
	m := mailer.sent[0]
	assert.Equal(t, "budi@mail.com", m.to)
	assert.Equal(t, "password-reset", m.template)
	assert.Equal(t, "Budi", m.data["Name"])
	assert.Equal(t, 30, m.data["Minutes"])
	raw := tokenFromLink(t, m.data["Link"].(string))
	assert.Len(t, raw, 43, "32 byte base64url tanpa padding")

	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	require.Contains(t, repo.tokens, hash)
	assert.NotContains(t, repo.tokens, raw, "token mentah tidak boleh disimpan")
	assert.WithinDuration(t, time.Now().Add(30*time.Minute), repo.tokens[hash].expiresAt, 5*time.Second)
}

func TestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	svc, repo, mailer := newResetSvc()

	err := svc.RequestReset(context.Background(), "tidak@ada.com", "1.2.3.4")

	assert.NoError(t, err, "tidak boleh membedakan email terdaftar dan tidak")
	assert.Empty(t, mailer.sent)
	assert.Empty(t, repo.tokens)
}

func TestPasswordReset_NewRequestRevokesOldToken(t *testing.T) {
	svc, _, mailer := newResetSvc()
	ctx := context.Background()
	require.NoError(t, svc.RequestReset(ctx, "budi@mail.com", ""))
	require.NoError(t, svc.RequestReset(ctx, "budi@mail.com", ""))
	oldTok := tokenFromLink(t, mailer.sent[0].data["Link"].(string))
	newTok := tokenFromLink(t, mailer.sent[1].data["Link"].(string))

	assert.ErrorIs(t, svc.ResetPassword(ctx, oldTok, "passwordbaru1"), apperror.ErrInvalidResetToken)
	assert.NoError(t, svc.ResetPassword(ctx, newTok, "passwordbaru1"))
}

func TestPasswordReset_LimitPerHourPerAccount(t *testing.T) {
	svc, _, mailer := newResetSvc()
	for i := 0; i < 6; i++ {
		assert.NoError(t, svc.RequestReset(context.Background(), "budi@mail.com", ""))
	}
	assert.Len(t, mailer.sent, 3, "permintaan ke-4 dan seterusnya diam-diam tidak dikirim")
}

func TestPasswordReset_MailFailureIsReturnedForLogging(t *testing.T) {
	svc, _, mailer := newResetSvc()
	mailer.err = errors.New("smtp down")

	assert.Error(t, svc.RequestReset(context.Background(), "budi@mail.com", ""))
}

func TestPasswordReset_ResetSetsPasswordOnceAndRecordsChange(t *testing.T) {
	svc, repo, mailer := newResetSvc()
	ctx := context.Background()
	require.NoError(t, svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromLink(t, mailer.sent[0].data["Link"].(string))

	require.NoError(t, svc.ResetPassword(ctx, raw, "passwordbaru1"))

	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.passwords[7]), []byte("passwordbaru1")))
	assert.WithinDuration(t, time.Now(), repo.changedAt[7], 5*time.Second)
	assert.ErrorIs(t, svc.ResetPassword(ctx, raw, "passwordlain22"), apperror.ErrInvalidResetToken, "sekali pakai")
}

func TestPasswordReset_ExpiredTokenRejected(t *testing.T) {
	svc, repo, mailer := newResetSvc()
	ctx := context.Background()
	require.NoError(t, svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromLink(t, mailer.sent[0].data["Link"].(string))
	for _, tok := range repo.tokens {
		tok.expiresAt = time.Now().Add(-time.Second)
	}

	assert.ErrorIs(t, svc.ResetPassword(ctx, raw, "passwordbaru1"), apperror.ErrInvalidResetToken)
	assert.Empty(t, repo.passwords)
}

func TestPasswordReset_WeakPasswordDoesNotConsumeToken(t *testing.T) {
	svc, _, mailer := newResetSvc()
	ctx := context.Background()
	require.NoError(t, svc.RequestReset(ctx, "budi@mail.com", ""))
	raw := tokenFromLink(t, mailer.sent[0].data["Link"].(string))

	assert.ErrorIs(t, svc.ResetPassword(ctx, raw, "pendek"), apperror.ErrWeakPassword)
	assert.NoError(t, svc.ResetPassword(ctx, raw, "passwordbaru1"), "token masih berlaku setelah password ditolak")
}

func TestPasswordReset_UnknownOrEmptyTokenRejected(t *testing.T) {
	svc, _, _ := newResetSvc()
	for _, tok := range []string{"", "  ", "bukan-token"} {
		assert.ErrorIs(t, svc.ResetPassword(context.Background(), tok, "passwordbaru1"), apperror.ErrInvalidResetToken, tok)
	}
}

func TestValidatePassword(t *testing.T) {
	assert.ErrorIs(t, service.ValidatePassword("1234567"), apperror.ErrWeakPassword)
	assert.NoError(t, service.ValidatePassword("12345678"))
	assert.NoError(t, service.ValidatePassword(strings.Repeat("a", 72)))
	assert.ErrorIs(t, service.ValidatePassword(strings.Repeat("a", 73)), apperror.ErrWeakPassword, "bcrypt menolak lebih dari 72 byte")
	assert.NoError(t, service.ValidatePassword("pässwörd"), "8 karakter walau lebih dari 8 byte")
}

func TestRegister_RejectsWeakPasswordAndTokenCarriesIat(t *testing.T) {
	svc, repo := newSpecUserService()
	_, err := svc.Register(context.Background(), "a@b.com", "ab", "pendek")
	assert.ErrorIs(t, err, apperror.ErrWeakPassword)
	assert.Empty(t, repo.users, "akun tidak dibuat")

	tok, err := svc.Register(context.Background(), "a@b.com", "ab", "panjang123")
	require.NoError(t, err)
	assert.NotEmpty(t, parseClaim(t, tok, "iat"))
}
