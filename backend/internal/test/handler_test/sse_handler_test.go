package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"qms/internal/sse"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubQueueValidator struct {
	err error
}

func (s stubQueueValidator) ViewQueue(
	_ context.Context,
	queueID,
	requesterID int64,
) (model.Queue, bool, error) {
	return model.Queue{ID: queueID}, false, s.err
}

type lockedRecorder struct {
	mu   sync.Mutex
	hdr  http.Header
	code int
}

func (r *lockedRecorder) Header() http.Header         { return r.hdr }
func (r *lockedRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *lockedRecorder) WriteHeader(code int) {
	r.mu.Lock()
	r.code = code
	r.mu.Unlock()
}
func (r *lockedRecorder) Flush() {}

func TestServeSSE_UnsubscribesWhenClientDisconnects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := sse.NewHub()
	go hub.Run()

	engine := gin.New()
	engine.GET("/queues/:queue_id/event", handler.NewSSEHandler(hub, stubQueueValidator{}).ServeSSE)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/queues/5/event", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.ServeHTTP(&lockedRecorder{hdr: http.Header{}}, req)
	}()

	require.Eventually(t, func() bool {
		return hub.RoomSize(5) == 1
	}, time.Second, 5*time.Millisecond, "klien harus terdaftar di room")

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ServeSSE harus selesai setelah koneksi putus")
	}
	require.Eventually(t, func() bool {
		return hub.RoomSize(5) == 0
	}, time.Second, 5*time.Millisecond, "klien harus dilepas dari room setelah putus")
}

func TestServeSSE_RejectsBeforeSubscribing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := sse.NewHub()
	go hub.Run()

	engine := gin.New()
	engine.GET("/queues/:queue_id/event",
		handler.NewSSEHandler(hub, stubQueueValidator{err: apperror.ErrNotFound}).ServeSSE)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/queues/5/event", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 0, hub.RoomSize(5))
}

func TestServeSSE_SetsStreamingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := sse.NewHub()
	go hub.Run()

	engine := gin.New()
	engine.GET("/queues/:queue_id/event", handler.NewSSEHandler(hub, stubQueueValidator{}).ServeSSE)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/queues/5/event", nil).WithContext(ctx)
	w := &lockedRecorder{hdr: http.Header{}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.ServeHTTP(w, req)
	}()

	require.Eventually(t, func() bool { return hub.RoomSize(5) == 1 }, time.Second, 5*time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}
