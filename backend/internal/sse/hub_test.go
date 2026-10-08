package sse

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBroadcast_IsolatesRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	c1 := &Client{hub: hub, send: make(chan []byte, 4), queueID: 1}
	c2 := &Client{hub: hub, send: make(chan []byte, 4), queueID: 1}
	c3 := &Client{hub: hub, send: make(chan []byte, 4), queueID: 2}

	hub.Subscribe(c1)
	hub.Subscribe(c2)
	hub.Subscribe(c3)
	time.Sleep(100 * time.Millisecond)

	hub.Broadcast(1, MarshalEvent(1, EventTicketJoined, map[string]any{"n": 1}))
	time.Sleep(100 * time.Millisecond)

	for _, c := range []*Client{c1, c2} {
		select {
		case raw := <-c.send:
			var e Event
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatalf("invalid event json: %v", err)
			}
			if e.Type != EventTicketJoined || e.QueueID != 1 {
				t.Fatalf("unexpected event: %+v", e)
			}
		default:
			t.Fatal("expected message for room 1 client")
		}
	}
	select {
	case msg := <-c3.send:
		t.Fatalf("room 2 client should not receive room 1 message: %s", msg)
	default:
	}
}