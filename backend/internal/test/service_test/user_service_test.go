package service_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeUserRepo struct {
	createUser model.User
	createErr  error
	findUser   model.User
	findErr    error
	// findUsers dipakai FindByLogin; bila kosong dan findErr nil, dianggap tidak ditemukan.
	findUsers []model.User
	// lastIdentifier merekam argumen FindByLogin terakhir.
	lastIdentifier string
	isOwner        bool
	findByIDErr    error
	expiresAt      *time.Time
	gotUsername    string
	gotEmailPref   *bool
	prefErr        error
}

func (r *fakeUserRepo) SubscriptionExpiresAt(context.Context, int64) (*time.Time, error) {
	return r.expiresAt, nil
}

func (r *fakeUserRepo) SetEmailNotifications(_ context.Context, _ int64, enabled bool) error {
	r.gotEmailPref = &enabled
	return r.prefErr
}

func (r *fakeUserRepo) UpdateUsername(_ context.Context, id int64, username string) (model.User, error) {
	r.gotUsername = username
	return model.User{ID: id, Username: username}, nil
}

func (r *fakeUserRepo) FindByID(
	ctx context.Context,
	id int64,
) (model.User, error) {
	if r.findByIDErr != nil {
		return model.User{}, r.findByIDErr
	}
	return model.User{ID: id, Email: "me@test.com", Username: "me"}, nil
}

func (r *fakeUserRepo) IsOwner(
	ctx context.Context,
	userID int64,
) (bool, error) {
	return r.isOwner, nil
}

func (r *fakeUserRepo) FindByLogin(
	ctx context.Context,
	identifier string,
) ([]model.User, error) {
	r.lastIdentifier = identifier
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.findUsers == nil {
		return []model.User{r.findUser}, nil
	}
	return r.findUsers, nil
}

func (r *fakeUserRepo) Create(
	ctx context.Context,
	email,
	username,
	passwordHash string,
) (model.User, error) {
	return r.createUser, r.createErr
}

func TestProfile_ReportsOwnerFlag(t *testing.T) {
	svc := service.NewUserService(&fakeUserRepo{isOwner: true})
	p, err := svc.Profile(context.Background(), 7)
	assert.NoError(t, err)
	assert.Equal(t, int64(7), p.User.ID)
	assert.True(t, p.IsOwner)

	svc = service.NewUserService(&fakeUserRepo{isOwner: false})
	p, err = svc.Profile(context.Background(), 8)
	assert.NoError(t, err)
	assert.False(t, p.IsOwner)
	assert.Nil(t, p.SubscriptionExpiresAt)
}

func TestProfile_SubscriptionExpiryOnlyForOwner(t *testing.T) {
	exp := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	svc := service.NewUserService(&fakeUserRepo{isOwner: true, expiresAt: &exp})
	p, err := svc.Profile(context.Background(), 7)
	assert.NoError(t, err)
	if assert.NotNil(t, p.SubscriptionExpiresAt) {
		assert.True(t, exp.Equal(*p.SubscriptionExpiresAt))
	}

	// Customer: expiry tidak dibaca walau repo mengembalikan nilai.
	svc = service.NewUserService(&fakeUserRepo{isOwner: false, expiresAt: &exp})
	p, err = svc.Profile(context.Background(), 8)
	assert.NoError(t, err)
	assert.Nil(t, p.SubscriptionExpiresAt)
}

func TestUpdateUsername_TrimsAndValidatesLength(t *testing.T) {
	repo := &fakeUserRepo{}
	svc := service.NewUserService(repo)

	u, err := svc.UpdateUsername(context.Background(), 7, "  Budi Baru  ")
	assert.NoError(t, err)
	assert.Equal(t, "Budi Baru", u.Username)
	assert.Equal(t, "Budi Baru", repo.gotUsername)

	for _, bad := range []string{"", "   ", strings.Repeat("a", 51)} {
		repo.gotUsername = ""
		_, err = svc.UpdateUsername(context.Background(), 7, bad)
		assert.ErrorIs(t, err, apperror.ErrInvalidUsername, "%q", bad)
		assert.Empty(t, repo.gotUsername, "repo tidak boleh dipanggil untuk input tidak valid")
	}

	// Batas dihitung per karakter, bukan per byte.
	_, err = svc.UpdateUsername(context.Background(), 7, strings.Repeat("é", 50))
	assert.NoError(t, err)
}

// fakeAvatarStore: AvatarStore di memori.
type fakeAvatarStore struct {
	data    map[int64][]byte
	updated map[int64]time.Time
	putErr  error
}

func newFakeAvatarStore() *fakeAvatarStore {
	return &fakeAvatarStore{data: map[int64][]byte{}, updated: map[int64]time.Time{}}
}

func (f *fakeAvatarStore) Put(_ context.Context, id int64, data []byte) (time.Time, error) {
	if f.putErr != nil {
		return time.Time{}, f.putErr
	}
	f.data[id] = data
	f.updated[id] = time.Now()
	return f.updated[id], nil
}

func (f *fakeAvatarStore) Get(_ context.Context, id int64) (model.Avatar, error) {
	d, ok := f.data[id]
	if !ok {
		return model.Avatar{}, apperror.ErrNotFound
	}
	return model.Avatar{Data: d, ContentType: "image/jpeg", UpdatedAt: f.updated[id]}, nil
}

func (f *fakeAvatarStore) Delete(_ context.Context, id int64) error {
	delete(f.data, id)
	delete(f.updated, id)
	return nil
}

func (f *fakeAvatarStore) UpdatedAt(_ context.Context, id int64) (*time.Time, error) {
	t, ok := f.updated[id]
	if !ok {
		return nil, nil
	}
	return &t, nil
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	assert.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))))
	return b.Bytes()
}

func TestAvatar_SetStoresProcessedJPEGAndProfileReportsIt(t *testing.T) {
	store := newFakeAvatarStore()
	svc := service.NewUserService(&fakeUserRepo{}).WithAvatars(store)
	ctx := context.Background()

	p, err := svc.Profile(ctx, 7)
	assert.NoError(t, err)
	assert.Nil(t, p.AvatarUpdatedAt, "belum ada foto")

	assert.NoError(t, svc.SetAvatar(ctx, 7, bytes.NewReader(pngBytes(t, 800, 600))))

	a, err := svc.Avatar(ctx, 7)
	assert.NoError(t, err)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(a.Data))
	assert.NoError(t, err, "yang tersimpan JPEG hasil olahan, bukan PNG asli")
	assert.Equal(t, 512, cfg.Width)
	assert.Equal(t, 512, cfg.Height)

	p, err = svc.Profile(ctx, 7)
	assert.NoError(t, err)
	assert.NotNil(t, p.AvatarUpdatedAt)
}

func TestAvatar_InvalidUploadsMapToAppErrorsAndStoreNothing(t *testing.T) {
	store := newFakeAvatarStore()
	svc := service.NewUserService(&fakeUserRepo{}).WithAvatars(store)
	ctx := context.Background()

	assert.ErrorIs(t, svc.SetAvatar(ctx, 7, strings.NewReader("bukan gambar")), apperror.ErrUnsupportedImage)
	assert.ErrorIs(t, svc.SetAvatar(ctx, 7, bytes.NewReader(append(pngBytes(t, 4, 4)[:30], 0))), apperror.ErrInvalidImage)
	assert.ErrorIs(t, svc.SetAvatar(ctx, 7, strings.NewReader(strings.Repeat("a", 6<<20))), apperror.ErrImageTooLarge)
	assert.Empty(t, store.data)
}

func TestAvatar_ReplaceDeleteAndDeleteTwice(t *testing.T) {
	store := newFakeAvatarStore()
	svc := service.NewUserService(&fakeUserRepo{}).WithAvatars(store)
	ctx := context.Background()

	assert.NoError(t, svc.SetAvatar(ctx, 7, bytes.NewReader(pngBytes(t, 50, 50))))
	assert.NoError(t, svc.SetAvatar(ctx, 7, bytes.NewReader(pngBytes(t, 60, 60))))
	assert.Len(t, store.data, 1, "satu foto per user")

	assert.NoError(t, svc.DeleteAvatar(ctx, 7))
	_, err := svc.Avatar(ctx, 7)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.NoError(t, svc.DeleteAvatar(ctx, 7), "hapus ganda tidak galat")
}

func TestAvatar_WithoutStoreBehavesAsNoPhoto(t *testing.T) {
	svc := service.NewUserService(&fakeUserRepo{})
	ctx := context.Background()
	p, err := svc.Profile(ctx, 7)
	assert.NoError(t, err)
	assert.Nil(t, p.AvatarUpdatedAt)
	_, err = svc.Avatar(ctx, 7)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.NoError(t, svc.DeleteAvatar(ctx, 7))
}

func TestSetEmailNotifications_PassesValueThroughBothWays(t *testing.T) {
	repo := &fakeUserRepo{}
	svc := service.NewUserService(repo)
	for _, want := range []bool{false, true} {
		assert.NoError(t, svc.SetEmailNotifications(context.Background(), 7, want))
		if assert.NotNil(t, repo.gotEmailPref) {
			assert.Equal(t, want, *repo.gotEmailPref)
		}
	}
	repo.prefErr = apperror.ErrNotFound
	assert.ErrorIs(t, svc.SetEmailNotifications(context.Background(), 7, true), apperror.ErrNotFound)
}
