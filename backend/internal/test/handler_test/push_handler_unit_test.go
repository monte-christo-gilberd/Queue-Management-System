package handler_test

import (
	"context"
	"net/http"
	"testing"

	"qms/internal/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePushStore struct {
	userID   int64
	endpoint string
	deleted  string
}

func (f *fakePushStore) Upsert(_ context.Context, userID int64, endpoint, _, _ string) error {
	f.userID, f.endpoint = userID, endpoint
	return nil
}

func (f *fakePushStore) Delete(_ context.Context, userID int64, endpoint string) error {
	f.userID, f.deleted = userID, endpoint
	return nil
}

func pushEngine(store *fakePushStore) (e interface{}, h *handler.PushHandler) {
	return nil, handler.NewPushHandler(store, "PUBKEY")
}

func TestPush_PublicKey(t *testing.T) {
	e := specEngine(7)
	_, h := pushEngine(&fakePushStore{})
	e.GET("/push/public-key", h.PublicKey)

	w := specRequest(e, http.MethodGet, "/push/public-key", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "PUBKEY", specDecode(t, w)["data"].(map[string]interface{})["public_key"])
}

func TestPush_SubscribeStoresForCurrentUser(t *testing.T) {
	store := &fakePushStore{}
	e := specEngine(7)
	_, h := pushEngine(store)
	e.POST("/push/subscribe", h.Subscribe)

	w := specRequest(e, http.MethodPost, "/push/subscribe",
		`{"endpoint":"https://fcm.googleapis.com/x","keys":{"p256dh":"a","auth":"b"}}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), store.userID)
	assert.Equal(t, "https://fcm.googleapis.com/x", store.endpoint)
}

func TestPush_SubscribeRejectsBadInput(t *testing.T) {
	for _, body := range []string{
		`nope`,
		`{"endpoint":"https://x.y","keys":{"p256dh":"a"}}`,
		`{"endpoint":"http://x.y","keys":{"p256dh":"a","auth":"b"}}`,
		`{"endpoint":"javascript:alert(1)","keys":{"p256dh":"a","auth":"b"}}`,
	} {
		store := &fakePushStore{}
		e := specEngine(7)
		_, h := pushEngine(store)
		e.POST("/push/subscribe", h.Subscribe)
		w := specRequest(e, http.MethodPost, "/push/subscribe", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Empty(t, store.endpoint)
	}
}

func TestPush_UnsubscribeOnlyOwnEndpoint(t *testing.T) {
	store := &fakePushStore{}
	e := specEngine(9)
	_, h := pushEngine(store)
	e.POST("/push/unsubscribe", h.Unsubscribe)

	w := specRequest(e, http.MethodPost, "/push/unsubscribe", `{"endpoint":"https://x.y/z"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(9), store.userID)
	assert.Equal(t, "https://x.y/z", store.deleted)
}
