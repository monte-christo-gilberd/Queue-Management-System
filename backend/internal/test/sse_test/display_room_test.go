package sse_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"qms/internal/sse"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotify_DisplayRoomGetsOnlyDatalessSignal(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	normal := connect(t, ctx, hub, 1)
	display := connect(t, ctx, hub, sse.DisplayRoom(1))
	other := connect(t, ctx, hub, sse.DisplayRoom(2))
	require.Eventually(t, func() bool { return hub.RoomSize(1) == 1 && hub.RoomSize(sse.DisplayRoom(1)) == 1 }, time.Second, 5*time.Millisecond)

	hub.Notify(1, sse.EventTicketNext, map[string]interface{}{"user_id": 4242, "ticket_number": 7})

	require.Eventually(t, func() bool { return strings.Contains(display.String(), "data: ") }, time.Second, 5*time.Millisecond)
	events := parseEvents(t, display.String())
	require.Len(t, events, 1)
	assert.Equal(t, sse.EventDisplayChanged, events[0].Type)
	assert.Nil(t, events[0].Payload, "layar publik tidak boleh menerima payload tiket")
	assert.NotContains(t, display.String(), "4242")

	require.Eventually(t, func() bool { return strings.Contains(normal.String(), "4242") }, time.Second, 5*time.Millisecond, "room biasa tetap menerima payload lengkap")
	assert.Never(t, func() bool { return strings.Contains(other.String(), "data: ") }, 100*time.Millisecond, 10*time.Millisecond, "layar antrian lain tidak menerima")
}

func TestNotify_QRUpdateDoesNotWakeDisplays(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	display := connect(t, ctx, hub, sse.DisplayRoom(1))
	require.Eventually(t, func() bool { return hub.RoomSize(sse.DisplayRoom(1)) == 1 }, time.Second, 5*time.Millisecond)

	hub.Notify(1, "queue_qr_updated", nil)

	assert.Never(t, func() bool { return strings.Contains(display.String(), "data: ") }, 100*time.Millisecond, 10*time.Millisecond)
}

func TestNotify_BeforeNotifyHookRunsFirst(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	display := connect(t, ctx, hub, sse.DisplayRoom(1))
	require.Eventually(t, func() bool { return hub.RoomSize(sse.DisplayRoom(1)) == 1 }, time.Second, 5*time.Millisecond)

	var sawEventAtHook bool
	hub.SetBeforeNotify(func(int64) { sawEventAtHook = strings.Contains(display.String(), "data: ") })
	hub.Notify(1, sse.EventTicketNext, nil)

	require.Eventually(t, func() bool { return strings.Contains(display.String(), "data: ") }, time.Second, 5*time.Millisecond)
	assert.False(t, sawEventAtHook, "hook (pembatalan cache) jalan sebelum event sampai ke klien")
}
