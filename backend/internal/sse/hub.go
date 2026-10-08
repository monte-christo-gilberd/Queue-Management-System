package sse

import "sync"

// Hub menyimpan koneksi per queue_id (room).
// Satu goroutine Run() memproses register/unregister/broadcast
// agar tidak ada race pada map.
type Hub struct {
	rooms      map[int64]map[*Client]bool // queue_id -> daftar koneksi yang "nonton" queue itu
	register   chan *Client               // antrean "mau gabung room ini"
	unregister chan *Client               // antrean "mau keluar dari room"
	broadcast  chan broadcastReq          // antrean "ada pesan buat disebar ke 1 room"
	mu         sync.RWMutex
}

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
	}
}

func (h *Hub) Run() {
	for {
		select {
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

// Subscribe mendaftarkan koneksi baru ke room queueID.
func (h *Hub) Subscribe(client *Client) bool {
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
func (h *Hub) Notify(queueID int64, action string, payload any) {
	h.Broadcast(queueID, MarshalEvent(queueID, action, payload))
}

// RoomSize untuk observasi/test.
func (h *Hub) RoomSize(queueID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[queueID])
}
