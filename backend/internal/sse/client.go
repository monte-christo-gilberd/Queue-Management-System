package sse

import (
	"fmt"
	"net/http"
	"time"
)

// DefaultHeartbeat: jeda komentar ": ping" agar koneksi idle tidak diputus proxy/load balancer.
const DefaultHeartbeat = 25 * time.Second

// Client adalah satu koneksi SSE dari browser atau aplikasi, terikat pada satu antrian.
type Client struct {
	// hub adalah hub tempat klien ini terdaftar; Serve memakai hub.done untuk tahu kapan server berhenti.
	hub *Hub
	// w adalah tulisan respons HTTP tempat event dikirim.
	w http.ResponseWriter
	// send adalah antrean pesan untuk klien ini. Hub yang mengisi, dan Serve yang mengambil dan menulis.
	// Kapasitas 256: bila penuh, klien dianggap terlalu lambat.
	send chan []byte
	// queueID adalah ID antrian yang ditonton klien ini (room-nya di Hub).
	queueID int64
	// heartbeat adalah jeda antar komentar ": ping".
	heartbeat time.Duration
}

// NewClient membuat Client untuk antrian queueID yang menulis ke w dan terdaftar di hub.
// Heartbeat memakai DefaultHeartbeat.
func NewClient(
	hub *Hub,
	w http.ResponseWriter,
	queueID int64,
) *Client {
	// isi Client dengan antrean pesan berkapasitas 256 dan heartbeat bawaan
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
	// simpan jeda baru lalu kembalikan Client yang sama agar bisa dirangkai
	c.heartbeat = d
	return c
}

// flush mengirim data yang sudah ditulis ke klien sekarang juga, tidak menunggu buffer penuh.
func (c *Client) flush() {
	// tidak semua ResponseWriter mendukung Flush; hanya dijalankan bila writer mendukungnya
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

	// ticker mengirim sinyal berkala untuk heartbeat
	ticker := time.NewTicker(c.heartbeat)
	// hentikan ticker saat Serve selesai agar tidak bocor
	defer ticker.Stop()

	// loop menunggu salah satu kejadian: pesan baru, waktu heartbeat, atau berhenti
	for {
		select {
		// ada pesan di antrean klien
		case message, ok := <-c.send:
			// channel sudah ditutup oleh hub (klien di-unregister karena terlalu lambat): hentikan
			if !ok {
				return
			}
			// tulis pesan sebagai satu event SSE ("data: ...") lalu kirim ke klien
			fmt.Fprintf(c.w, "data: %s\n\n", message)
			c.flush()
		// saatnya heartbeat: kirim komentar ": ping" agar koneksi tetap hidup
		case <-ticker.C:
			fmt.Fprint(c.w, ": ping\n\n")
			c.flush()
		// server sedang shutdown (hub.done ditutup): hentikan agar http.Server.Shutdown tidak menunggu
		case <-c.hub.done:
			return
		// klien sudah memutus koneksi (konteks request selesai): hentikan
		case <-r.Context().Done():
			return
		}
	}
}
