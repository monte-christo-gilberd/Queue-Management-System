package service_test

import (
	"context"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"strings"
	"sync"
	"testing"
	"time"

	"database/sql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVerifyRepo meniru semantik SQL: token sekali pakai, kedaluwarsa, token lama dibatalkan saat token baru dibuat.
type fakeVerifyRepo struct {
	mu     sync.Mutex
	users  map[int64]*model.User
	tokens map[string]*fakeToken // kunci: hash
}

func newFakeVerifyRepo(users ...model.User) *fakeVerifyRepo {
	r := &fakeVerifyRepo{users: map[int64]*model.User{}, tokens: map[string]*fakeToken{}}
	for i := range users {
		u := users[i]
		r.users[u.ID] = &u
	}
	return r
}

func (r *fakeVerifyRepo) FindForVerification(_ context.Context, userID int64) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[userID]
	if !ok {
		return model.User{}, apperror.ErrNotFound
	}
	return *u, nil
}

func (r *fakeVerifyRepo) IsVerified(_ context.Context, userID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[userID]
	if !ok {
		return false, apperror.ErrNotFound
	}
	return u.EmailVerifiedAt.Valid, nil
}

func (r *fakeVerifyRepo) CountRecentTokens(_ context.Context, userID int64, purpose string, since time.Time) (int, error) {
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

func (r *fakeVerifyRepo) CreateToken(_ context.Context, userID int64, purpose, hash string, exp time.Time, _ string) error {
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

func (r *fakeVerifyRepo) VerifyEmail(_ context.Context, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[hash]
	if !ok || t.purpose != service.PurposeEmailVerify || t.usedAt != nil || !t.expiresAt.After(time.Now()) {
		return apperror.ErrInvalidResetToken
	}
	now := time.Now()
	t.usedAt = &now
	if u := r.users[t.userID]; !u.EmailVerifiedAt.Valid {
		u.EmailVerifiedAt = sql.NullTime{Time: now, Valid: true}
	}
	return nil
}

func newVerifySvc() (*service.EmailVerificationService, *fakeVerifyRepo, *fakeResetMailer) {
	repo := newFakeVerifyRepo(
		model.User{ID: 7, Email: "budi@mail.com", Username: "Budi"},
		model.User{ID: 8, Email: "sari@mail.com", Username: "Sari", EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true}},
	)
	mailer := &fakeResetMailer{}
	return service.NewEmailVerificationService(repo, mailer, "https://app.example/"), repo, mailer
}

// tokenFromMail mengambil token mentah dari tautan di email terkirim.
func tokenFromMail(t *testing.T, m sentMail) string {
	t.Helper()
	link, ok := m.data["Link"].(string)
	require.True(t, ok)
	const prefix = "https://app.example/verify-email?token="
	require.True(t, strings.HasPrefix(link, prefix), link)
	return strings.TrimPrefix(link, prefix)
}

func TestSendVerification_SendsLinkAndStoresOnlyHash(t *testing.T) {
	svc, repo, mailer := newVerifySvc()

	require.NoError(t, svc.SendVerification(context.Background(), 7, "1.2.3.4"))

	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "budi@mail.com", mailer.sent[0].to)
	assert.Equal(t, "email-verify", mailer.sent[0].template)
	assert.Equal(t, "Budi", mailer.sent[0].data["Name"])
	assert.Equal(t, 24, mailer.sent[0].data["Hours"])
	raw := tokenFromMail(t, mailer.sent[0])
	assert.GreaterOrEqual(t, len(raw), 40)
	assert.NotContains(t, repo.tokens, raw, "token mentah tidak boleh disimpan, hanya hash-nya")
	require.Len(t, repo.tokens, 1)
}

func TestSendVerification_AlreadyVerifiedSendsNothing(t *testing.T) {
	svc, repo, mailer := newVerifySvc()

	require.NoError(t, svc.SendVerification(context.Background(), 8, ""))

	assert.Empty(t, mailer.sent)
	assert.Empty(t, repo.tokens)
}

func TestSendVerification_UnknownUserIsNotFound(t *testing.T) {
	svc, _, mailer := newVerifySvc()

	err := svc.SendVerification(context.Background(), 999, "")

	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Empty(t, mailer.sent)
}

func TestSendVerification_LimitedToThreePerHour(t *testing.T) {
	svc, _, mailer := newVerifySvc()
	ctx := context.Background()
	for range 3 {
		require.NoError(t, svc.SendVerification(ctx, 7, ""))
	}

	err := svc.SendVerification(ctx, 7, "")

	assert.ErrorIs(t, err, apperror.ErrTooManyVerifyEmails)
	assert.Len(t, mailer.sent, 3, "permintaan keempat tidak mengirim email")
}

func TestSendVerification_NewLinkInvalidatesOldOne(t *testing.T) {
	svc, _, mailer := newVerifySvc()
	ctx := context.Background()
	require.NoError(t, svc.SendVerification(ctx, 7, ""))
	require.NoError(t, svc.SendVerification(ctx, 7, ""))
	require.Len(t, mailer.sent, 2)

	assert.ErrorIs(t, svc.Verify(ctx, tokenFromMail(t, mailer.sent[0])), apperror.ErrInvalidResetToken)
	assert.NoError(t, svc.Verify(ctx, tokenFromMail(t, mailer.sent[1])))
}

func TestSendVerification_MailFailureIsReturned(t *testing.T) {
	svc, _, mailer := newVerifySvc()
	mailer.err = errors.New("smtp down")

	assert.Error(t, svc.SendVerification(context.Background(), 7, ""))
}

func TestVerify_MarksEmailVerifiedAndTokenIsSingleUse(t *testing.T) {
	svc, repo, mailer := newVerifySvc()
	ctx := context.Background()
	require.NoError(t, svc.SendVerification(ctx, 7, ""))
	raw := tokenFromMail(t, mailer.sent[0])
	require.ErrorIs(t, svc.RequireVerified(ctx, 7), apperror.ErrEmailNotVerified)

	require.NoError(t, svc.Verify(ctx, " "+raw+" "), "spasi di tepi dibuang")

	assert.True(t, repo.users[7].EmailVerifiedAt.Valid)
	assert.NoError(t, svc.RequireVerified(ctx, 7))
	assert.ErrorIs(t, svc.Verify(ctx, raw), apperror.ErrInvalidResetToken, "token tidak bisa dipakai dua kali")
}

func TestVerify_RejectsEmptyUnknownAndExpiredTokens(t *testing.T) {
	svc, repo, mailer := newVerifySvc()
	ctx := context.Background()
	require.NoError(t, svc.SendVerification(ctx, 7, ""))
	raw := tokenFromMail(t, mailer.sent[0])
	for _, tok := range repo.tokens {
		tok.expiresAt = time.Now().Add(-time.Minute)
	}

	assert.ErrorIs(t, svc.Verify(ctx, ""), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, svc.Verify(ctx, "bukan-token"), apperror.ErrInvalidResetToken)
	assert.ErrorIs(t, svc.Verify(ctx, raw), apperror.ErrInvalidResetToken, "token kedaluwarsa")
	assert.False(t, repo.users[7].EmailVerifiedAt.Valid)
}

func TestRequireVerified_ReflectsAccountState(t *testing.T) {
	svc, _, _ := newVerifySvc()
	ctx := context.Background()

	assert.ErrorIs(t, svc.RequireVerified(ctx, 7), apperror.ErrEmailNotVerified)
	assert.NoError(t, svc.RequireVerified(ctx, 8))
	assert.ErrorIs(t, svc.RequireVerified(ctx, 999), apperror.ErrNotFound)
}

// --- UserService: registrasi dan profil ---

type fakeVerifier struct {
	called chan int64
	err    error
}

func (f *fakeVerifier) SendVerification(_ context.Context, userID int64, _ string) error {
	f.called <- userID
	return f.err
}

func TestRegister_SendsVerificationEmailInBackground(t *testing.T) {
	svc, repo := newSpecUserService()
	verifier := &fakeVerifier{called: make(chan int64, 1)}
	svc.WithEmailVerifier(verifier)

	token, err := svc.Register(context.Background(), "baru@example.com", "baru", "rahasia123")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	select {
	case id := <-verifier.called:
		assert.Equal(t, repo.users[0].ID, id)
	case <-time.After(2 * time.Second):
		t.Fatal("email verifikasi tidak dikirim setelah registrasi")
	}
}

func TestRegister_SucceedsEvenWhenVerificationEmailFails(t *testing.T) {
	svc, _ := newSpecUserService()
	verifier := &fakeVerifier{called: make(chan int64, 1), err: errors.New("smtp down")}
	svc.WithEmailVerifier(verifier)

	token, err := svc.Register(context.Background(), "baru@example.com", "baru", "rahasia123")

	require.NoError(t, err)
	assert.NotEmpty(t, token, "kegagalan kirim email tidak menggagalkan registrasi")
	<-verifier.called
}

func TestRegister_DuplicateEmailSendsNoVerification(t *testing.T) {
	svc, _ := newSpecUserService()
	ctx := context.Background()
	_, err := svc.Register(ctx, "dup@example.com", "first", "rahasia123")
	require.NoError(t, err)
	verifier := &fakeVerifier{called: make(chan int64, 1)}
	svc.WithEmailVerifier(verifier)

	_, err = svc.Register(ctx, "dup@example.com", "second", "lainnya456")

	assert.ErrorIs(t, err, apperror.ErrDuplicate)
	select {
	case <-verifier.called:
		t.Fatal("registrasi yang gagal tidak boleh mengirim email verifikasi")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestProfile_EmailVerifiedFlag(t *testing.T) {
	ctx := context.Background()

	t.Run("verifikasi mati: selalu terverifikasi", func(t *testing.T) {
		svc, repo := newSpecUserService()
		_, err := svc.Register(ctx, "a@example.com", "a", "rahasia123")
		require.NoError(t, err)
		profile, err := svc.Profile(ctx, repo.users[0].ID)
		require.NoError(t, err)
		assert.True(t, profile.EmailVerified)
	})

	t.Run("verifikasi menyala: ikut status akun", func(t *testing.T) {
		svc, repo := newSpecUserService()
		verifier := &fakeVerifier{called: make(chan int64, 4)}
		svc.WithEmailVerifier(verifier)
		_, err := svc.Register(ctx, "a@example.com", "a", "rahasia123")
		require.NoError(t, err)
		<-verifier.called
		id := repo.users[0].ID

		profile, err := svc.Profile(ctx, id)
		require.NoError(t, err)
		assert.False(t, profile.EmailVerified)

		repo.users[0].EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		profile, err = svc.Profile(ctx, id)
		require.NoError(t, err)
		assert.True(t, profile.EmailVerified)
	})
}

// --- Guard di join dan buat antrian ---

type fakeGuard struct {
	mu       sync.Mutex
	verified map[int64]bool
}

func (g *fakeGuard) RequireVerified(_ context.Context, userID int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.verified[userID] {
		return apperror.ErrEmailNotVerified
	}
	return nil
}

func (g *fakeGuard) set(userID int64, v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.verified[userID] = v
}

func TestJoin_UnverifiedUserIsRejectedWithoutTicket(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{}}
	env.ts.WithVerificationGuard(guard)

	_, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)
	list, err := env.ts.ListTickets(context.Background(), env.queueID, env.ownerID, nil)
	require.NoError(t, err)
	assert.Empty(t, list, "tidak ada tiket yang dibuat")
	assert.Empty(t, env.events.snapshot(), "tidak ada event yang disiarkan")
}

func TestJoin_VerifiedUserJoinsNormally(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{specCustomerA: true}}
	env.ts.WithVerificationGuard(guard)

	ticket, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, 1, ticket.TicketNumber)
}

func TestJoin_RejoinReturnsExistingTicketEvenIfNoLongerVerified(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{specCustomerA: true}}
	env.ts.WithVerificationGuard(guard)
	first, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)
	require.NoError(t, err)
	guard.set(specCustomerA, false)

	again, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	require.NoError(t, err, "scan ulang tidak berubah menjadi galat")
	assert.Equal(t, first.ID, again.ID)
}

func TestJoin_NoGuardMeansNoCheck(t *testing.T) {
	env := newSpecEnv(t)

	_, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	assert.NoError(t, err)
}

func TestCreateQueue_UnverifiedOwnerIsRejectedBeforeSubscriptionCheck(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{}}
	env.qs.WithVerificationGuard(guard)
	// pemilik tanpa langganan sekalipun mendapat ErrEmailNotVerified lebih dulu, bukan ErrSubscriptionInactive
	const noSubscription int64 = 555

	_, err := env.qs.CreateQueue(context.Background(), noSubscription, "Loket", "")
	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)

	_, err = env.qs.CreateQueue(context.Background(), env.ownerID, "Loket 2", "")
	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)

	guard.set(env.ownerID, true)
	_, err = env.qs.CreateQueue(context.Background(), env.ownerID, "Loket 2", "")
	assert.NoError(t, err)

	guard.set(noSubscription, true)
	_, err = env.qs.CreateQueue(context.Background(), noSubscription, "Loket", "")
	assert.ErrorIs(t, err, apperror.ErrSubscriptionInactive, "setelah terverifikasi, aturan langganan berlaku seperti biasa")
}
