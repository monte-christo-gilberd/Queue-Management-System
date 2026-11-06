package sse_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"qms/internal/sse"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncRecorder adalah http.ResponseWriter yang aman dipakai lintas goroutine
// (Serve menulis dari goroutine sendiri, tes membaca dari goroutine tes).
type syncRecorder struct {
	mu  sync.Mutex
	buf bytes.Buffer
	hdr http.Header
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{hdr: http.Header{}}
}

func (r *syncRecorder) Header() http.Header { return r.hdr }

func (r *syncRecorder) WriteHeader(int) {}

func (r *syncRecorder) Flush() {}

func (r *syncRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(b)
}

func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// connect mendaftarkan satu klien SSE ke queueID dan mulai Serve di goroutine terpisah.
func connect(t *testing.T, ctx context.Context, hub *sse.Hub, queueID int64) *syncRecorder {
	t.Helper()

	w := newSyncRecorder()
	client := sse.NewClient(hub, w, queueID)
	require.True(t, hub.Subscribe(client))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	go client.Serve(req)

	return w
}

func parseEvents(t *testing.T, raw string) []sse.Event {
	t.Helper()

	var events []sse.Event
	for _, frame := range strings.Split(raw, "\n\n") {
		frame = strings.TrimSpace(frame)
		if frame == "" || strings.HasPrefix(frame, ":") { // komentar SSE (": connected", ": ping")
			continue
		}
		require.True(t, strings.HasPrefix(frame, "data: "), "frame SSE harus diawali 'data: ': %q", frame)

		var e sse.Event
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(frame, "data: ")), &e))
		events = append(events, e)
	}
	return events
}

func TestBroadcast_IsolatesRooms(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c1 := connect(t, ctx, hub, 1)
	c2 := connect(t, ctx, hub, 1)
	c3 := connect(t, ctx, hub, 2)

	require.Eventually(t, func() bool {
		return hub.RoomSize(1) == 2 && hub.RoomSize(2) == 1
	}, time.Second, 5*time.Millisecond, "klien harus terdaftar di room masing-masing")

	hub.Broadcast(1, sse.MarshalEvent(1, sse.EventTicketJoined, map[string]interface{}{"n": 1}))

	for name, c := range map[string]*syncRecorder{"c1": c1, "c2": c2} {
		require.Eventually(t, func() bool {
			return strings.Contains(c.String(), "data: ")
		}, time.Second, 5*time.Millisecond, "%s harus menerima pesan room 1", name)

		events := parseEvents(t, c.String())
		require.Len(t, events, 1, name)
		assert.Equal(t, sse.EventTicketJoined, events[0].Type, name)
		assert.Equal(t, int64(1), events[0].QueueID, name)
	}

	// Beri waktu broadcast selesai sebelum memastikan room 2 tidak menerima apa pun.
	assert.Never(t, func() bool {
		return strings.Contains(c3.String(), "data: ")
	}, 100*time.Millisecond, 10*time.Millisecond, "klien room 2 tidak boleh menerima pesan room 1")
}

func TestServe_SendsOpeningCommentAndHeartbeat(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := newSyncRecorder()
	client := sse.NewClient(hub, w, 1).WithHeartbeat(20 * time.Millisecond)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	go client.Serve(req)

	// Byte pertama keluar segera (tanpa menunggu event) supaya fetch() klien resolve.
	require.Eventually(t, func() bool {
		return strings.HasPrefix(w.String(), ": connected\n\n")
	}, time.Second, 2*time.Millisecond)

	// Lalu heartbeat berkala menjaga koneksi idle tetap hidup.
	require.Eventually(t, func() bool {
		return strings.Contains(w.String(), ": ping\n\n")
	}, time.Second, 5*time.Millisecond)

	assert.Empty(t, parseEvents(t, w.String()), "komentar bukan event")
}

func TestHubShutdown_EndsServeAndRejectsNewClients(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()

	w := newSyncRecorder()
	client := sse.NewClient(hub, w, 1)
	require.True(t, hub.Subscribe(client))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	served := make(chan struct{})
	go func() {
		client.Serve(req)
		close(served)
	}()

	hub.Shutdown()

	select {
	case <-served:
	case <-time.After(time.Second):
		t.Fatal("Serve harus berhenti setelah Hub.Shutdown")
	}
	assert.False(t, hub.Subscribe(sse.NewClient(hub, newSyncRecorder(), 1)), "klien baru ditolak setelah shutdown")
	assert.NotPanics(t, hub.Shutdown, "Shutdown kedua tidak boleh panik")
}
