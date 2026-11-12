package sse

import "sync"

// Hub menyimpan koneksi per queue_id (room).
// Satu goroutine Run() memproses register/unregister/broadcast
// agar tidak ada race pada map. Run berjalan sampai Shutdown dipanggil (saat server berhenti).
type Hub struct {
	// rooms: kunci room -> himpunan koneksi yang menonton room itu. Kuncinya queue_id, atau DisplayRoom(queue_id)
	// untuk layar publik. Hanya Run yang mengubah isinya.
	rooms map[int64]map[*Client]bool
	// register: antrean klien yang mau bergabung ke room-nya (diisi Subscribe, kapasitas 256).
	register chan *Client
	// unregister: antrean klien yang mau keluar dari room-nya (diisi remove, kapasitas 256).
	unregister chan *Client
	// broadcast: antrean pesan yang harus disebar ke satu room (diisi Broadcast, kapasitas 1024).
	broadcast chan broadcastReq
	// mu: melindungi rooms. Run memakai Lock saat mengubah, RLock saat menyebar; RoomSize memakai RLock
	// karena dipanggil dari goroutine lain (observasi dan tes).
	mu sync.RWMutex

	// beforeNotify dipanggil di Notify SEBELUM event disebar. Dipakai membatalkan cache layar publik supaya
	// klien yang mengambil ulang setelah menerima sinyal tidak mendapat snapshot lama. Diisi sekali saat start.
	beforeNotify func(queueID int64)

	// done ditutup oleh Shutdown. Client.Serve berhenti saat done tertutup agar http.Server.Shutdown tidak
	// menunggu koneksi SSE yang berumur panjang; Run berhenti, dan Subscribe menolak klien baru.
	done chan struct{}
	// stopOnce memastikan done hanya ditutup sekali, walaupun Shutdown dipanggil berkali-kali.
	stopOnce sync.Once
}

// DisplayRoom: kunci room untuk layar publik antrian ini. Layar publik TIDAK ikut room antrian biasa karena
// room itu membawa payload tiket lengkap (id user, dst); room ini hanya menerima sinyal tanpa data.
// Kuncinya dibuat negatif agar tidak bentrok dengan queue_id yang selalu positif.
func DisplayRoom(queueID int64) int64 { return -queueID }

// SetBeforeNotify memasang hook yang dipanggil di Notify sebelum tiap event disebar (mis. membatalkan cache layar
// publik). Tidak aman dipanggil bersamaan dengan Notify, jadi panggil sekali saat start sebelum ada lalu lintas.
func (h *Hub) SetBeforeNotify(fn func(queueID int64)) { h.beforeNotify = fn }

// broadcastReq adalah satu permintaan siaran: pesan dan room tujuannya.
type broadcastReq struct {
	// queueID adalah kunci room yang menerima pesan.
	queueID int64
	// message adalah isi pesan yang sudah dalam bentuk JSON.
	message []byte
}

// NewHub membuat Hub baru dengan semua channel dan map sudah siap dipakai. Hub belum berjalan sampai Run dipanggil.
func NewHub() *Hub {
	// buat map room kosong dan channel dengan kapasitas tertentu
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
	// tutup channel done hanya sekali; pemanggilan berikutnya diabaikan oleh stopOnce
	h.stopOnce.Do(func() { close(h.done) })
}

// Run memproses register, unregister, dan broadcast dalam satu goroutine sampai Shutdown dipanggil.
// Panggil di goroutine terpisah, mis. go hub.Run(); hanya boleh dijalankan satu kali per Hub.
// Selama Run belum berjalan, register/unregister/broadcast hanya menumpuk di antrean channel.
func (h *Hub) Run() {
	// ulangi terus, menunggu salah satu channel berisi pekerjaan
	for {
		select {
		// hub dihentikan: keluar dari loop
		case <-h.done:
			return
		// ada klien yang mau bergabung ke room
		case client := <-h.register:
			// kunci map karena akan diubah
			h.mu.Lock()
			// ambil room antrian klien; bila belum ada, buat room baru
			room, ok := h.rooms[client.queueID]
			if !ok {
				room = make(map[*Client]bool)
				h.rooms[client.queueID] = room
			}
			// daftarkan klien ke room
			room[client] = true
			h.mu.Unlock()
		// ada klien yang mau keluar dari room
		case client := <-h.unregister:
			// kunci map karena akan diubah
			h.mu.Lock()
			// hanya lanjut bila room antrian klien ini ada
			if room, ok := h.rooms[client.queueID]; ok {
				// hanya lanjut bila klien memang terdaftar di room itu (hindari dua kali unregister)
				if _, exists := room[client]; exists {
					// hapus klien dari room dan tutup channel send-nya agar Serve berhenti
					delete(room, client)
					close(client.send)
					// room kosong: hapus dari map agar tidak menumpuk
					if len(room) == 0 {
						delete(h.rooms, client.queueID)
					}
				}
			}
			h.mu.Unlock()
		// ada pesan yang harus disebar ke satu room
		case req := <-h.broadcast:
			// kunci baca saja karena hanya membaca daftar klien
			h.mu.RLock()
			// ambil daftar klien di room tujuan
			room := h.rooms[req.queueID]
			// kirim pesan ke setiap klien di room
			for client := range room {
				select {
				// klien masih bisa menerima pesan: kirim
				case client.send <- req.message:
				// antrean klien penuh (klien lambat): jangan block broadcast ke yang lain.
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

// remove mengirim klien ke antrean unregister tanpa menunggu. Mengembalikan false bila antrean penuh.
func (h *Hub) remove(client *Client) bool {
	select {
	// antrean unregister masih punya tempat: kirim klien
	case h.unregister <- client:
		return true
	// antrean penuh: batalkan, pengirim tidak boleh macet
	default:
		return false
	}
}

// Subscribe mendaftarkan koneksi baru ke room queueID. Mengembalikan false bila antrean register penuh atau hub
// sudah di-Shutdown; handler membalas 503 untuk kasus ini.
func (h *Hub) Subscribe(client *Client) bool {
	// cek lebih dulu apakah hub sudah dihentikan; bila ya, tolak klien baru
	select {
	case <-h.done:
		return false
	default:
	}
	// coba masukkan klien ke antrean register tanpa menunggu
	select {
	// berhasil masuk antrean
	case h.register <- client:
		return true
	// antrean penuh
	default:
		return false
	}
}

// Unsubscribe melepas koneksi dari room. Tidak menunggu: bila antrean unregister penuh, permintaan dibuang dan
// klien tetap terdaftar sampai terlepas lewat jalur lain (mis. dianggap lambat saat Broadcast). Hasil remove diabaikan.
func (h *Hub) Unsubscribe(client *Client) {
	// kirim ke antrean unregister; hasil (berhasil atau tidak) diabaikan
	h.remove(client)
}

// Broadcast mengirim pesan mentah (sudah berupa JSON) ke semua klien di room queueID. Pesan kosong diabaikan.
// Tidak pernah memblokir pemanggil: pesan diantre ke channel broadcast, dan dibuang bila antrean penuh
// (klien menutup celahnya lewat resinkron berkala).
func (h *Hub) Broadcast(queueID int64, message []byte) {
	// pesan kosong tidak perlu dikirim
	if len(message) == 0 {
		return
	}
	// coba antre pesan; bila antrean broadcast penuh, pesan dibuang agar pemanggil tidak menunggu
	select {
	case h.broadcast <- broadcastReq{queueID: queueID, message: message}:
	default:
	}
}

// Notify memenuhi interface service.EventNotifier tanpa import cycle:
// service mendefinisikan interface Notify(queueID, action, payload),
// Hub menyediakannya secara struktural.
//
// Urutan kerjanya: hook beforeNotify, lalu event lengkap (dengan payload) ke room antrian, lalu sinyal
// EventDisplayChanged tanpa data ke room layar publik. Aksi EventQueueQRUpdated tidak memicu sinyal layar publik.
func (h *Hub) Notify(queueID int64, action string, payload interface{}) {
	// bila hook beforeNotify dipasang, jalankan dulu (misalnya membatalkan cache layar publik)
	if h.beforeNotify != nil {
		h.beforeNotify(queueID)
	}
	// sebarkan event lengkap ke klien di room antrian ini
	h.Broadcast(queueID, MarshalEvent(queueID, action, payload))
	// Perubahan QR tidak mengubah apa pun di layar publik.
	// untuk aksi selain perubahan QR, kirim juga sinyal tanpa data ke layar publik
	if action != EventQueueQRUpdated {
		h.Broadcast(DisplayRoom(queueID), MarshalEvent(queueID, EventDisplayChanged, nil))
	}
}

// RoomSize mengembalikan jumlah klien di room queueID (0 bila room tidak ada). Aman dipanggil dari goroutine mana pun.
// Dipakai untuk observasi dan tes.
func (h *Hub) RoomSize(queueID int64) int {
	// kunci baca agar jumlah tidak dibaca saat map sedang diubah
	h.mu.RLock()
	// buka kunci saat fungsi selesai
	defer h.mu.RUnlock()
	// jumlah klien di room; room yang tidak ada menghasilkan 0
	return len(h.rooms[queueID])
}

// ClientCounts mengembalikan jumlah koneksi aktif: queue untuk room antrian (kunci positif) dan display untuk
// layar publik (kunci negatif, lihat DisplayRoom). Aman dipanggil dari goroutine mana pun; dipakai metrik.
func (h *Hub) ClientCounts() (queue int, display int) {
	// kunci baca agar map tidak berubah saat dihitung
	h.mu.RLock()
	// buka kunci saat fungsi selesai
	defer h.mu.RUnlock()
	// jumlahkan klien per room sesuai jenis kuncinya
	for key, room := range h.rooms {
		if key < 0 {
			display += len(room)
		} else {
			queue += len(room)
		}
	}
	return queue, display
}
