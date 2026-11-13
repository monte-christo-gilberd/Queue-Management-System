package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const qrPublicAppURL = "https://app.example.com"

// seqCodes: pembuat kode yang mengembalikan kode dari daftar secara berurutan (kode valid, 6 karakter).
func seqCodes(codes ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		if i >= len(codes) {
			return "", errors.New("kode uji habis")
		}
		i++
		return codes[i-1], nil
	}
}

// newQRSpecEnv menyiapkan lingkungan uji dengan TTL dinamis 90 detik dan pembuat kode yang bisa diprediksi.
func newQRSpecEnv(t *testing.T, codes ...string) (*specEnv, *service.QRService) {
	t.Helper()
	env := newSpecEnv(t)
	svc := service.NewQRService(env.queues, qrPublicAppURL, env.events).
		WithDynamicTTL(90 * time.Second).
		WithCodeGenerator(seqCodes(codes...))
	return env, svc
}

// expireQRCode memundurkan masa berlaku kode dinamis antrian ke masa lalu.
func expireQRCode(env *specEnv, queueID int64) {
	env.queues.mu.Lock()
	defer env.queues.mu.Unlock()
	settings := env.queues.settings[queueID]
	settings.QRExpiresAt.Time = time.Now().Add(-time.Minute)
	settings.QRExpiresAt.Valid = true
	env.queues.settings[queueID] = settings
}

// Kode statis diterbitkan sekali, disimpan, dan dipakai lagi pada GET berikutnya tanpa event.
func TestQR_GetQR_StaticCodeIssuedOnceAndPersisted(t *testing.T) {
	env, svc := newQRSpecEnv(t, "STAT23")
	ctx := context.Background()

	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, "queue_qr", first.Action)
	assert.Equal(t, env.queueID, first.QueueID)
	assert.Equal(t, model.QueueQRTypeStatic, first.Type)
	assert.Nil(t, first.ExpiresAt)
	assert.Equal(t, qrPublicAppURL+"/join?t=STAT23", first.JoinURL)

	// Kode statis tidak dibuat ulang: generator hanya punya satu kode.
	second, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, first.JoinURL, second.JoinURL)
	assert.Equal(t, "STAT23", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated), "GET tidak mengirim event")
}

// Kode dinamis tetap dipakai selama belum kedaluwarsa, lalu diterbitkan kode baru setelah masa berlakunya habis.
func TestQR_GetQR_DynamicCodeKeptUntilExpiredOrJoined(t *testing.T) {
	env, svc := newQRSpecEnv(t, "DYN234", "DYN345")
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)

	before := time.Now().Unix()
	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeDynamic, first.Type)
	require.NotNil(t, first.ExpiresAt)
	assert.GreaterOrEqual(t, *first.ExpiresAt, before+89)
	assert.LessOrEqual(t, *first.ExpiresAt, before+91)
	assert.Equal(t, "DYN234", env.queues.currentQRCode(env.queueID))

	// Tanpa join atau kedaluwarsa, kode yang sama tetap dipakai.
	second, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, first.JoinURL, second.JoinURL)
	assert.Equal(t, "DYN234", env.queues.currentQRCode(env.queueID))

	// Setelah kedaluwarsa, GET menerbitkan kode baru.
	expireQRCode(env, env.queueID)
	third, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, qrPublicAppURL+"/join?t=DYN345", third.JoinURL)
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

// Antrian lama yang belum punya baris queue_settings dibuatkan pengaturan default (statis) saat pertama dibuka.
// Kode statis berformat lama (token panjang dari sebelum kode 6 karakter) diganti dengan kode join baru saat dibuka.
func TestQR_GetQR_ReplacesLegacyLongToken(t *testing.T) {
	env, svc := newQRSpecEnv(t, "FRESH2")
	ctx := context.Background()
	env.queues.mu.Lock()
	settings := env.queues.settings[env.queueID]
	settings.QRCurrentCode = sql.NullString{String: "v1.eyJxaWQiOjF9.sig", Valid: true}
	env.queues.settings[env.queueID] = settings
	env.queues.mu.Unlock()

	resp, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, qrPublicAppURL+"/join?t=FRESH2", resp.JoinURL)
	assert.Equal(t, "FRESH2", env.queues.currentQRCode(env.queueID))
}

func TestQR_GetQR_BackfillsMissingSettings(t *testing.T) {
	// Antrian lama (dibuat sebelum fitur QR) belum punya baris queue_settings.
	env, svc := newQRSpecEnv(t, "BACK23")
	ctx := context.Background()
	env.queues.dropSettings(env.queueID)

	resp, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, resp.Type)
	assert.Equal(t, "BACK23", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
}

// Non-owner tidak boleh melihat kode QR antrian; respons disamarkan sebagai ErrNotFound.
func TestQR_GetQR_NonOwnerIsNotFound(t *testing.T) {
	env, svc := newQRSpecEnv(t, "OWNR23")

	_, err := svc.GetQR(context.Background(), env.queueID, specOtherOwnerID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Empty(t, env.queues.currentQRCode(env.queueID))
}

// Galat dari pembuat kode diteruskan dan tidak ada kode yang tersimpan.
func TestQR_GetQR_GeneratorErrorPropagates(t *testing.T) {
	env := newSpecEnv(t)
	svc := service.NewQRService(env.queues, qrPublicAppURL, env.events).
		WithCodeGenerator(func() (string, error) { return "", errors.New("rng mati") })

	_, err := svc.GetQR(context.Background(), env.queueID, env.ownerID)
	assert.Error(t, err)
	assert.Empty(t, env.queues.currentQRCode(env.queueID))
}

// Mengubah mode menyimpan tipe baru, mengirim tepat satu event, dan kode statis baru diterbitkan saat kembali dari dinamis.
func TestQR_SetQRType_PersistsAndEmitsOnce(t *testing.T) {
	env, svc := newQRSpecEnv(t, "DYNA23", "STAT34")
	ctx := context.Background()

	resp, err := svc.SetQRType(ctx, env.queueID, env.ownerID, model.QueueQRTypeDynamic)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeDynamic, resp.Type)
	require.NotNil(t, resp.ExpiresAt)
	assert.Equal(t, model.QueueQRTypeDynamic, env.queues.qrType(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))

	// Kembali ke statis: kode dinamis yang masih tersimpan tidak dipakai sebagai statis; diterbitkan kode statis baru.
	resp, err = svc.SetQRType(ctx, env.queueID, env.ownerID, model.QueueQRTypeStatic)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Nil(t, resp.ExpiresAt)
	assert.Equal(t, qrPublicAppURL+"/join?t=STAT34", resp.JoinURL)
	assert.Equal(t, 2, env.events.count(service.EventQueueQRUpdated))
}

// Tipe tidak dikenal dan non-owner ditolak tanpa mengubah pengaturan atau mengirim event.
func TestQR_SetQRType_RejectsUnknownTypeAndNonOwner(t *testing.T) {
	env, svc := newQRSpecEnv(t, "NOPE23")
	ctx := context.Background()

	_, err := svc.SetQRType(ctx, env.queueID, env.ownerID, "hologram")
	assert.ErrorIs(t, err, apperror.ErrQRTypeInvalid)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))

	_, err = svc.SetQRType(ctx, env.queueID, specOtherOwnerID, model.QueueQRTypeDynamic)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, model.QueueQRTypeStatic, env.queues.qrType(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

// Mengganti kode statis membuat kode lama tidak berlaku dan mengirim satu event.
func TestQR_ReplaceQR_IssuesNewStaticCodeAndEmits(t *testing.T) {
	// penggantian kode membuat kode lama langsung tidak berlaku.
	env, svc := newQRSpecEnv(t, "OLDD23", "NEWC34")
	ctx := context.Background()

	first, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)

	replaced, err := svc.ReplaceQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, model.QueueQRTypeStatic, replaced.Type)
	assert.Nil(t, replaced.ExpiresAt)
	assert.NotEqual(t, first.JoinURL, replaced.JoinURL)
	assert.Equal(t, "NEWC34", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))

	_, err = svc.ResolveJoinToken(ctx, "OLDD23")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid, "kode lama harus ditolak")

	queue, err := svc.ResolveJoinToken(ctx, "NEWC34")
	require.NoError(t, err)
	assert.Equal(t, env.queueID, queue.ID)
}

// Kode dinamis tidak bisa diganti manual, dan non-owner tidak boleh mengganti kode.
func TestQR_ReplaceQR_DynamicIsRejectedAndNonOwner(t *testing.T) {
	env, svc := newQRSpecEnv(t, "DYNR23")
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)

	_, err := svc.ReplaceQR(ctx, env.queueID, env.ownerID)
	assert.ErrorIs(t, err, apperror.ErrQRReplaceNotAllowed)
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))

	_, err = svc.ReplaceQR(ctx, env.queueID, specOtherOwnerID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

// Kode yang diketik boleh huruf kecil dan spasi, sedangkan format yang salah atau karakter yang dikecualikan ditolak.
func TestQR_ResolveJoinToken_AcceptsTypedCodeAndRejectsBadFormat(t *testing.T) {
	env, svc := newQRSpecEnv(t, "TYPE23")
	ctx := context.Background()
	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)

	// Kode yang diketik boleh huruf kecil dan dikelilingi spasi.
	queue, err := svc.ResolveJoinToken(ctx, "  type23 ")
	require.NoError(t, err)
	assert.Equal(t, env.queueID, queue.ID)

	for _, bad := range []string{"", "TYPE2", "TYPE234", "TYP0 23", "TYPE2O", "TYPE2I", "nope99"} {
		_, err = svc.ResolveJoinToken(ctx, bad)
		assert.ErrorIs(t, err, apperror.ErrQRInvalid, "kode %q harus ditolak", bad)
	}
}

// Kode dinamis yang sudah kedaluwarsa ditolak dengan ErrQRExpired.
func TestQR_ResolveJoinToken_DynamicExpiredIsGone(t *testing.T) {
	env, svc := newQRSpecEnv(t, "EXPR23")
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)
	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)

	_, err = svc.ResolveJoinToken(ctx, "EXPR23")
	require.NoError(t, err)

	expireQRCode(env, env.queueID)
	_, err = svc.ResolveJoinToken(ctx, "EXPR23")
	assert.ErrorIs(t, err, apperror.ErrQRExpired)
}

// Join dengan kode dinamis yang aktif memutar kode; join yang memakai kode lama tidak memutar lagi.
func TestQR_AfterJoin_RotatesDynamicCodeOnlyWhenActive(t *testing.T) {
	env, svc := newQRSpecEnv(t, "ACTV23", "NXTV34", "LATF34")
	ctx := context.Background()
	env.queues.setQRType(env.queueID, model.QueueQRTypeDynamic)
	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)

	// Join berhasil dengan kode aktif: kode diganti dan event dikirim.
	require.NoError(t, svc.AfterJoin(ctx, env.queueID, "actv23"))
	assert.Equal(t, "NXTV34", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))

	// Kode lama tidak berlaku lagi.
	_, err = svc.ResolveJoinToken(ctx, "ACTV23")
	assert.ErrorIs(t, err, apperror.ErrQRInvalid)

	// Join lain yang masih memakai kode lama tidak memutar kode baru lagi.
	require.NoError(t, svc.AfterJoin(ctx, env.queueID, "ACTV23"))
	assert.Equal(t, "NXTV34", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 1, env.events.count(service.EventQueueQRUpdated))
}

// Kode statis tidak pernah berganti karena join.
func TestQR_AfterJoin_StaticCodeNeverRotates(t *testing.T) {
	env, svc := newQRSpecEnv(t, "STAY23")
	ctx := context.Background()
	_, err := svc.GetQR(ctx, env.queueID, env.ownerID)
	require.NoError(t, err)

	require.NoError(t, svc.AfterJoin(ctx, env.queueID, "STAY23"))
	assert.Equal(t, "STAY23", env.queues.currentQRCode(env.queueID))
	assert.Equal(t, 0, env.events.count(service.EventQueueQRUpdated))
}

// Garis miring di akhir URL dasar dibuang sebelum membentuk tautan join.
func TestQR_JoinURL_TrimsTrailingSlash(t *testing.T) {
	env := newSpecEnv(t)
	svc := service.NewQRService(env.queues, qrPublicAppURL+"/", env.events).
		WithCodeGenerator(seqCodes("PATH23"))

	resp, err := svc.GetQR(context.Background(), env.queueID, env.ownerID)
	require.NoError(t, err)
	assert.Equal(t, qrPublicAppURL+"/join?t=PATH23", resp.JoinURL)
}
