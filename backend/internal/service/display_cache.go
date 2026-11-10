package service

import (
	"qms/internal/model"
	"sync"
	"time"
)

// DisplayCache menyimpan snapshot layar publik per token selama ttl, dan menggabungkan pemuatan yang bersamaan
// (satu query untuk banyak layar). Invalidate(queueID) dipanggil Hub SEBELUM event disebar, jadi klien yang mengambil
// ulang setelah sinyal tidak pernah mendapat snapshot yang lebih lama dari event itu.
type DisplayCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	byToken map[string]*displayEntry
	byQueue map[int64]map[string]struct{}
	loading map[string]*displayCall
	// epoch naik di tiap Invalidate. Pemuatan yang mulai sebelum Invalidate tidak boleh disimpan atau dipakai
	// bersama oleh permintaan yang datang sesudahnya.
	epoch uint64
}

// displayEntry: satu snapshot yang tersimpan di cache, lengkap dengan antrian dan waktu kedaluwarsanya.
type displayEntry struct {
	queueID int64
	snap    model.DisplaySnapshot
	expires time.Time
}

// displayCall: satu pemuatan snapshot yang sedang berjalan. Permintaan lain untuk token yang sama menunggu
// wg, lalu memakai hasilnya tanpa query tambahan.
type displayCall struct {
	wg      sync.WaitGroup
	epoch   uint64
	queueID int64
	snap    model.DisplaySnapshot
	err     error
}

// NewDisplayCache membuat cache kosong dengan umur snapshot ttl. Waktu bisa diganti lewat field now saat tes.
func NewDisplayCache(ttl time.Duration) *DisplayCache {
	return &DisplayCache{
		ttl:     ttl,
		now:     time.Now,
		byToken: map[string]*displayEntry{},
		byQueue: map[int64]map[string]struct{}{},
		loading: map[string]*displayCall{},
	}
}

// Get mengembalikan snapshot untuk token; `load` hanya dijalankan bila tidak ada entri segar.
func (c *DisplayCache) Get(token string, load func() (int64, model.DisplaySnapshot, error)) (int64, model.DisplaySnapshot, error) {
	// Kunci mutex agar pembacaan dan penulisan cache tidak bertabrakan dengan goroutine lain.
	c.mu.Lock()
	// Jika ada entri untuk token dan belum kedaluwarsa, kembalikan langsung tanpa memuat ulang.
	if e, ok := c.byToken[token]; ok && c.now().Before(e.expires) {
		// Lepas kunci sebelum mengembalikan hasil dari cache.
		c.mu.Unlock()
		return e.queueID, e.snap, nil
	}
	// Sudah ada pemuatan yang sedang berjalan untuk token ini dan masih di epoch yang sama: ikut menunggu hasilnya.
	if call, ok := c.loading[token]; ok && call.epoch == c.epoch {
		// Lepas kunci agar pemuatan yang sedang berjalan bisa selesai dan menyimpan hasilnya.
		c.mu.Unlock()
		// Tunggu sampai pemuatan yang sedang berjalan selesai.
		call.wg.Wait()
		return call.queueID, call.snap, call.err
	}
	call := &displayCall{epoch: c.epoch}
	// Tandai satu pemuatan yang akan ditunggu oleh permintaan lain.
	call.wg.Add(1)
	c.loading[token] = call
	// Lepas kunci sebelum memanggil load, karena load bisa memakan waktu (query database).
	c.mu.Unlock()

	// Jalankan fungsi load untuk mengambil snapshot dari database.
	call.queueID, call.snap, call.err = load()

	// Kunci lagi untuk membereskan status pemuatan dan menyimpan hasil.
	c.mu.Lock()
	// Hapus tanda pemuatan hanya jika masih milik panggilan ini (belum diganti oleh yang lain).
	if c.loading[token] == call {
		delete(c.loading, token)
	}
	// Simpan hasil hanya jika berhasil dan tidak ada Invalidate di tengah jalan (epoch masih sama).
	if call.err == nil && call.epoch == c.epoch {
		// Simpan snapshot ke cache dengan umur ttl.
		c.store(token, call.queueID, call.snap)
	}
	// Lepas kunci setelah cache selesai diperbarui.
	c.mu.Unlock()
	// Beri tahu semua permintaan yang menunggu bahwa hasil sudah siap.
	call.wg.Done()
	return call.queueID, call.snap, call.err
}

// QueueID: antrian pemilik token bila ada entri segar (tanpa menyentuh database).
func (c *DisplayCache) QueueID(token string) (int64, bool) {
	// Kunci mutex, dan lepaskan otomatis saat fungsi selesai.
	c.mu.Lock()
	defer c.mu.Unlock()
	// Cek entri untuk token; hanya dianggap valid bila belum kedaluwarsa.
	if e, ok := c.byToken[token]; ok && c.now().Before(e.expires) {
		return e.queueID, true
	}
	return 0, false
}

// Invalidate membuang semua entri antrian ini (termasuk token lama yang sudah diganti).
func (c *DisplayCache) Invalidate(queueID int64) {
	// Kunci mutex, dan lepaskan otomatis saat fungsi selesai.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	// Hapus setiap token yang terdaftar untuk antrian ini dari daftar entri.
	for token := range c.byQueue[queueID] {
		delete(c.byToken, token)
	}
	delete(c.byQueue, queueID)
}

func (c *DisplayCache) store(token string, queueID int64, snap model.DisplaySnapshot) {
	// Cache sudah terlalu besar: buang dulu entri yang sudah kedaluwarsa agar memori tidak terus membesar.
	if len(c.byToken) > 1000 {
		now := c.now()
		// Periksa setiap entri dan hapus yang sudah kedaluwarsa dari kedua peta.
		for t, e := range c.byToken {
			// Entri sudah tidak berlaku lagi (waktu sekarang tidak sebelum waktu kedaluwarsanya): buang.
			if !now.Before(e.expires) {
				delete(c.byToken, t)
				delete(c.byQueue[e.queueID], t)
			}
		}
	}
	// Simpan snapshot baru dengan waktu kedaluwarsa sekarang ditambah ttl.
	c.byToken[token] = &displayEntry{queueID: queueID, snap: snap, expires: c.now().Add(c.ttl)}
	// Buat daftar token untuk antrian ini bila belum ada, agar token bisa dicari saat Invalidate.
	if c.byQueue[queueID] == nil {
		c.byQueue[queueID] = map[string]struct{}{}
	}
	c.byQueue[queueID][token] = struct{}{}
}
