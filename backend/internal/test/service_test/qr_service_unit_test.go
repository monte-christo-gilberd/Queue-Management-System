package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
	qrclient "qms/internal/qrclient"
	"qms/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const qrPublicAppURL = "https://app.example.com"

// fakeQRClient: fake service.QRClient. Token berurutan agar mudah dibedakan
// (static-1, static-2, ...). Tidak menandatangani apa pun — itu tugas qr-service.
type fakeQRClient struct {
	mu sync.Mutex

	issueCount int
	issueCalls []fakeIssueCall
	issueErr   error

	verifyTokens []string
	verifyClaims qrclient.JoinTokenClaims
	verifyErr    error
}

type fakeIssueCall struct {
	QueueID    int64
	Type       string
	TTLSeconds int
}

func (f *fakeQRClient) IssueJoinToken(
	_ context.Context,
	queueID int64,
	typ string,
	ttlSeconds int,
) (qrclient.JoinToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issueCount++
	f.issueCalls = append(f.issueCalls, fakeIssueCall{QueueID: queueID, Type: typ, TTLSeconds: ttlSeconds})
	if f.issueErr != nil {
		return qrclient.JoinToken{}, f.issueErr
	}

	out := qrclient.JoinToken{
		Token: fmt.Sprintf("%s-%d", typ, f.issueCount),
		Type:  typ,
	}
	if typ == model.QueueQRTypeDynamic {
		exp := time.Now().Add(time.Duration(ttlSeconds) * time.Second).Unix()
		out.ExpiresAt = &exp
	}
	return out, nil
}

func (f *fakeQRClient) VerifyJoinToken(
	_ context.Context,
	token string,
) (qrclient.JoinTokenClaims, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verifyTokens = append(f.verifyTokens, token)
	if f.verifyErr != nil {
		return qrclient.JoinTokenClaims{}, f.verifyErr
	}
	return f.verifyClaims, nil
}

func (f *fakeQRClient) issued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issueCount
}

func (f *fakeQRClient) calls() []fakeIssueCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeIssueCall(nil), f.issueCalls...)
}

func newQRSpecEnv(t *testing.T) (*specEnv, *fakeQRClient, *service.QRService) {
	t.Helper()
	env := newSpecEnv(t)
	client := &fakeQRClient{}
	svc := service.NewQRService(env.queues, client, qrPublicAppURL, env.events).
		WithDynamicTTL(90 * time.Second)
	return env, client, svc
}

func TestQR_GetQR_StaticTokenIssuedOnceAndPersisted(t *testing.T) {
	// 4.2: QR statis dibuat sekali, disimpan, dan dipakai lagi pada permintaan berikutnya.
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, "queue_qr", first.Action)
	assert.Equal(t, env.queueID, first.QueueID)
	assert.Equal(t, model.QueueQRTypeStatic, first.Type)
	assert.Nil(t, first.ExpiresAt)
	assert.Equal(t, qrPublicAppURL+"/join?t=static-1", first.JoinURL)

	second, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, first.JoinURL, second.JoinURL)

	assert.Equal(t, 1, client.issued(), "token statis tidak boleh dibuat ulang setiap GET")
	assert.Equal(t, "static-1", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated), "GET tidak mengirim event")
}

func TestQR_GetQR_DynamicIssuesFreshTokenWithoutStoring(t *testing.T) {
	// 4.2: QR dinamis berganti tiap permintaan, tidak disimpan, dan punya masa berlaku.
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)

	before := time.Now().Unix()
	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeDynamic, first.Type)
	require.NotNil(t, first.ExpiresAt)
	assert.GreaterOrEqual(t, *first.ExpiresAt, before+89)
	assert.LessOrEqual(t, *first.ExpiresAt, before+91)

	second, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	require.NotNil(t, second.ExpiresAt)
	assert.NotEqual(t, first.JoinURL, second.JoinURL)

	assert.Equal(t, 2, client.issued())
	assert.Empty(t, env.queues.currentQRCode(env.queueID), "token dinamis tidak disimpan")
	for _, call := range client.calls() {
		assert.Equal(t, model.QueueQRTypeDynamic, call.Type)
		assert.Equal(t, 90, call.TTLSeconds)
	}
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

func TestQR_GetQR_BackfillsMissingSettings(t *testing.T) {
	// Antrian lama (dibuat sebelum fitur QR) belum punya baris queue_settings.
	env, _, svc := newQRSpecEnv(t)
	ctx := context.Background()
	env.queues.dropSettings(env.queueID)

	resp, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, resp.Type)
	assert.Equal(t, "static-1", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
}

func TestQR_GetQR_NonOwnerIsNotFound(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	_, err := svc.GetQR(ctx, env.queueID, specOtherOwnerID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, client.issued())
}

func TestQR_GetQR_PropagatesQRServiceError(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()
	client.issueErr = apperror.ErrQRUnavailable

	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	assert.ErrorIs(t, err, apperror.ErrQRUnavailable)
	assert.Empty(t, env.queues.currentQRCode(env.queueID))
}

func TestQR_SetQRType_PersistsAndEmitsOnce(t *testing.T) {
	env, _, svc := newQRSpecEnv(t)
	ctx := context.Background()

	resp, err := svc.SetQRType(ctx, env.queueID, env.ownerID, model.QueueQRTypeDynamic)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeDynamic, resp.Type)
	require.NotNil(t, resp.ExpiresAt)
	assert.Equal(t, model.QueueQRTypeDynamic, env.queues.qrType(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))

	// Kembali ke statis: token statis yang tersimpan (kalau ada) dipakai lagi.
	_, err = svc.SetQRType(ctx, env.queueID, env.ownerID, model.QueueQRTypeStatic)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Equal(t, 2, env.events.count(service.EventQueueQRUpdated))
}

func TestQR_SetQRType_RejectsUnknownTypeAndNonOwner(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	_, err := svc.SetQRType(ctx, env.queueID, env.ownerID, "hologram")
	assert.ErrorIs(t, err, apperror.ErrQRTypeInvalid)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Equal(t, 0, client.issued())
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))

	_, err = svc.SetQRType(ctx, env.queueID, specOtherOwnerID, model.QueueQRTypeDynamic)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

func TestQR_ReplaceQR_IssuesNewStaticTokenAndEmits(t *testing.T) {
	// 4.2: penggantian kode membuat token lama langsung tidak berlaku.
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	oldToken := env.queues.currentQRCode(env.queueID)

	replaced, err := svc.ReplaceQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, replaced.Type)
	assert.Nil(t, replaced.ExpiresAt)
	assert.NotEqual(t, first.JoinURL, replaced.JoinURL)
	assert.Equal(t, "static-2", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))

	// Token lama sudah tidak cocok dengan kode aktif.
	client.verifyClaims = qrclient.JoinTokenClaims{QueueID: env.queueID, Type: model.QueueQRTypeStatic}
	_, err = svc.ResolveJoinToken(ctx, oldToken)
	assert.ErrorIs(t, err, apperror.ErrQRInvalid)

	_, err = svc.ResolveJoinToken(ctx, "static-2")
	require.NoError(t, err)
}

func TestQR_ReplaceQR_DynamicIsRejected(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)

	_, err := svc.ReplaceQR(ctx, env.queueID, env.ownerID)
	assert.ErrorIs(t, err, apperror.ErrQRReplaceNotAllowed)
	assert.Equal(t, 0, client.issued())
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

func TestQR_ReplaceQR_NonOwnerIsNotFound(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	_, err := svc.ReplaceQR(ctx, env.queueID, specOtherOwnerID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, client.issued())
}

func TestQR_ResolveJoinToken_StaticMustMatchStoredCode(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	stored := env.queues.currentQRCode(env.queueID)

	client.verifyClaims = qrclient.JoinTokenClaims{QueueID: env.queueID, Type: model.QueueQRTypeStatic}
	queue, err := svc.ResolveJoinToken(ctx, stored)
	require.NoError(t, err)
	assert.Equal(t, env.queueID, queue.ID)

	// Tanda tangan valid tetapi bukan kode aktif (mis. QR lama) -> ditolak.
	_, err = svc.ResolveJoinToken(ctx, "static-lama")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid)
}

func TestQR_ResolveJoinToken_TypeMustMatchSettings(t *testing.T) {
	env, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	client.verifyClaims = qrclient.JoinTokenClaims{QueueID: env.queueID, Type: model.QueueQRTypeDynamic}
	_, err := svc.ResolveJoinToken(ctx, "dynamic-1")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid, "mode statis tidak menerima token dinamis")

	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)
	queue, err := svc.ResolveJoinToken(ctx, "dynamic-1")
	require.NoError(t, err)
	assert.Equal(t, env.queueID, queue.ID)
}

func TestQR_ResolveJoinToken_UnknownQueueAndClientErrors(t *testing.T) {
	_, client, svc := newQRSpecEnv(t)
	ctx := context.Background()

	client.verifyClaims = qrclient.JoinTokenClaims{QueueID: 999999, Type: model.QueueQRTypeStatic}
	_, err := svc.ResolveJoinToken(ctx, "static-1")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid)

	client.verifyErr = apperror.ErrQRExpired
	_, err = svc.ResolveJoinToken(ctx, "dynamic-1")
	assert.ErrorIs(t, err, apperror.ErrQRExpired)

	client.verifyErr = apperror.ErrQRInvalid
	_, err = svc.ResolveJoinToken(ctx, "rusak")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid)
}

func TestQR_JoinURL_TrimsTrailingSlash(t *testing.T) {
	env := newSpecEnv(t)
	client := &fakeQRClient{}
	svc := service.NewQRService(env.queues, client, qrPublicAppURL+"/", env.events)

	resp, err := svc.GetQR(context.Background(), env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, qrPublicAppURL+"/join?t=static-1", resp.JoinURL)
}
