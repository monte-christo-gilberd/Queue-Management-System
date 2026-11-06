package sse

import (
	"fmt"
	"net/http"
	"time"
)

// DefaultHeartbeat: jeda komentar ": ping" agar koneksi idle tidak diputus proxy/load balancer.
const DefaultHeartbeat = 25 * time.Second

type Client struct {
	hub       *Hub
	w         http.ResponseWriter
	send      chan []byte
	queueID   int64
	heartbeat time.Duration
}

func NewClient(
	hub *Hub,
	w http.ResponseWriter,
	queueID int64,
) *Client {
	return &Client{
		hub:     hub,
		w:       w,
		send:    make(chan []byte, 256),
		queueID: queueID,

		heartbeat: DefaultHeartbeat,
	}
}

// WithHeartbeat mengganti jeda heartbeat (untuk tes). Harus dipanggil sebelum Serve.
func (c *Client) WithHeartbeat(d time.Duration) *Client {
	c.heartbeat = d
	return c
}

func (c *Client) flush() {
	if flusher, ok := c.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Serve menulis event ke klien sampai koneksi putus (konteks request selesai), hub menutup channel send
// (klien terlalu lambat), atau hub di-Shutdown (server berhenti).
func (c *Client) Serve(r *http.Request) {
	// Komentar pembuka: memaksa header+body terkirim sehingga fetch() klien langsung resolve,
	// bukan menunggu event pertama.
	fmt.Fprint(c.w, ": connected\n\n")
	c.flush()

	ticker := time.NewTicker(c.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			fmt.Fprintf(c.w, "data: %s\n\n", message)
			c.flush()
		case <-ticker.C:
			fmt.Fprint(c.w, ": ping\n\n")
			c.flush()
		case <-c.hub.done:
			return
		case <-r.Context().Done():
			return
		}
	}
}
