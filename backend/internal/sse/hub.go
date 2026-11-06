package sse

import "sync"

// Hub menyimpan koneksi per queue_id (room).
// Satu goroutine Run() memproses register/unregister/broadcast
// agar tidak ada race pada map. Run berjalan sampai Shutdown dipanggil (saat server berhenti).
type Hub struct {
	rooms      map[int64]map[*Client]bool // queue_id -> daftar koneksi yang "nonton" queue itu
	register   chan *Client               // antrean "mau gabung room ini"
	unregister chan *Client               // antrean "mau keluar dari room"
	broadcast  chan broadcastReq          // antrean "ada pesan buat disebar ke 1 room"
	mu         sync.RWMutex

	// beforeNotify dipanggil di Notify SEBELUM event disebar. Dipakai membatalkan cache layar publik supaya
	// klien yang mengambil ulang setelah menerima sinyal tidak mendapat snapshot lama. Diisi sekali saat start.
	beforeNotify func(queueID int64)

	// done ditutup oleh Shutdown. Client.Serve berhenti saat done tertutup agar http.Server.Shutdown tidak
	// menunggu koneksi SSE yang berumur panjang; Run berhenti, dan Subscribe menolak klien baru.
	done     chan struct{}
	stopOnce sync.Once
}

// DisplayRoom: kunci room untuk layar publik antrian ini. Layar publik TIDAK ikut room antrian biasa karena
// room itu membawa payload tiket lengkap (id user, dst); room ini hanya menerima sinyal tanpa data.
func DisplayRoom(queueID int64) int64 { return -queueID }

// SetBeforeNotify memasang hook yang dipanggil sebelum tiap event disebar. Panggil sebelum ada lalu lintas.
func (h *Hub) SetBeforeNotify(fn func(queueID int64)) { h.beforeNotify = fn }

type broadcastReq struct {
	queueID int64
	message []byte
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[int64]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan broadcastReq, 1024),
		done:       make(chan struct{}),
	}
}

// Shutdown memberi tahu semua koneksi SSE untuk berhenti dan menghentikan Run. Aman dipanggil berkali-kali.
// Dipanggil saat server akan berhenti, sebelum http.Server.Shutdown.
func (h *Hub) Shutdown() {
	h.stopOnce.Do(func() { close(h.done) })
}

func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return
		case client := <-h.register:
			h.mu.Lock()
			room, ok := h.rooms[client.queueID]
			if !ok {
				room = make(map[*Client]bool)
				h.rooms[client.queueID] = room
			}
			room[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if room, ok := h.rooms[client.queueID]; ok {
				if _, exists := room[client]; exists {
					delete(room, client)
					close(client.send)
					if len(room) == 0 {
						delete(h.rooms, client.queueID)
					}
				}
			}
			h.mu.Unlock()
		case req := <-h.broadcast:
			h.mu.RLock()
			room := h.rooms[req.queueID]
			for client := range room {
				select {
				case client.send <- req.message:
				default:
					// Klien lambat: jangan block broadcast ke yang lain.
					// Tutup via unregister agar goroutine-nya selesai.
					go h.remove(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) remove(client *Client) bool {
	select {
	case h.unregister <- client:
		return true
	default:
		return false
	}
}

// Subscribe mendaftarkan koneksi baru ke room queueID. Mengembalikan false bila antrean register penuh atau hub
// sudah di-Shutdown; handler membalas 503 untuk kasus ini.
func (h *Hub) Subscribe(client *Client) bool {
	select {
	case <-h.done:
		return false
	default:
	}
	select {
	case h.register <- client:
		return true
	default:
		return false
	}
}

// Unsubscribe melepas koneksi dari room.
func (h *Hub) Unsubscribe(client *Client) {
	h.remove(client)
}

// Broadcast mengirim pesan mentah ke semua klien di satu queue.
// Non-blocking terhadap HTTP handler: antre ke channel, drop jika penuh.
func (h *Hub) Broadcast(queueID int64, message []byte) {
	if len(message) == 0 {
		return
	}
	select {
	case h.broadcast <- broadcastReq{queueID: queueID, message: message}:
	default:
	}
}

// Notify memenuhi interface service.EventNotifier tanpa import cycle:
// service mendefinisikan interface Notify(queueID, action, payload),
// Hub menyediakannya secara struktural.
func (h *Hub) Notify(queueID int64, action string, payload interface{}) {
	if h.beforeNotify != nil {
		h.beforeNotify(queueID)
	}
	h.Broadcast(queueID, MarshalEvent(queueID, action, payload))
	// Perubahan QR tidak mengubah apa pun di layar publik.
	if action != "queue_qr_updated" {
		h.Broadcast(DisplayRoom(queueID), MarshalEvent(queueID, EventDisplayChanged, nil))
	}
}

// RoomSize untuk observasi/test.
func (h *Hub) RoomSize(queueID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[queueID])
}
