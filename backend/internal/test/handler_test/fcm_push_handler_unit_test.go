package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"qms/internal/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDeviceStore: penyimpanan token palsu yang mencatat user, token, dan platform terakhir; err diisi untuk
// mensimulasikan database gagal.
type fakeDeviceStore struct {
	userID   int64
	token    string
	platform string
	deleted  string
	err      error
}

func (f *fakeDeviceStore) UpsertDevice(_ context.Context, userID int64, token, platform string) error {
	if f.err != nil {
		return f.err
	}
	f.userID, f.token, f.platform = userID, token, platform
	return nil
}

func (f *fakeDeviceStore) DeleteDevice(_ context.Context, userID int64, token string) error {
	if f.err != nil {
		return f.err
	}
	f.userID, f.deleted = userID, token
	return nil
}

// Register menyimpan token untuk user dari JWT (bukan dari body) dan menjawab 204.
func TestFCMPush_RegisterStoresForCurrentUser(t *testing.T) {
	store := &fakeDeviceStore{}
	e := specEngine(7)
	h := handler.NewFCMPushHandler(store)
	e.POST("/push/fcm/register", h.Register)

	w := specRequest(e, http.MethodPost, "/push/fcm/register", `{"token":"tok-1","platform":"android"}`)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(7), store.userID)
	assert.Equal(t, "tok-1", store.token)
	assert.Equal(t, "android", store.platform)
}

// Body rusak, token kosong atau spasi saja, platform kosong atau bukan android: 400 dan tidak ada yang disimpan.
func TestFCMPush_RegisterRejectsBadInput(t *testing.T) {
	for _, body := range []string{
		`nope`,
		`{"platform":"android"}`,
		`{"token":"","platform":"android"}`,
		`{"token":"   ","platform":"android"}`,
		`{"token":"tok"}`,
		`{"token":"tok","platform":"ios"}`,
	} {
		store := &fakeDeviceStore{}
		e := specEngine(7)
		h := handler.NewFCMPushHandler(store)
		e.POST("/push/fcm/register", h.Register)
		w := specRequest(e, http.MethodPost, "/push/fcm/register", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Empty(t, store.token, body)
	}
}

// Database gagal: 500 dengan pesan umum (detail galat tidak bocor ke klien).
func TestFCMPush_RegisterStoreError(t *testing.T) {
	e := specEngine(7)
	h := handler.NewFCMPushHandler(&fakeDeviceStore{err: errors.New("db down")})
	e.POST("/push/fcm/register", h.Register)
	w := specRequest(e, http.MethodPost, "/push/fcm/register", `{"token":"tok","platform":"android"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Unregister menghapus token atas nama user yang login (204), dan menolak body tanpa token (400).
func TestFCMPush_UnregisterOnlyOwnToken(t *testing.T) {
	store := &fakeDeviceStore{}
	e := specEngine(9)
	h := handler.NewFCMPushHandler(store)
	e.POST("/push/fcm/unregister", h.Unregister)

	w := specRequest(e, http.MethodPost, "/push/fcm/unregister", `{"token":"tok-1"}`)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(9), store.userID)
	assert.Equal(t, "tok-1", store.deleted)

	w = specRequest(e, http.MethodPost, "/push/fcm/unregister", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
