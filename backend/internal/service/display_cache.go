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

type displayEntry struct {
	queueID int64
	snap    model.DisplaySnapshot
	expires time.Time
}

type displayCall struct {
	wg      sync.WaitGroup
	epoch   uint64
	queueID int64
	snap    model.DisplaySnapshot
	err     error
}

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
	c.mu.Lock()
	if e, ok := c.byToken[token]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.queueID, e.snap, nil
	}
	if call, ok := c.loading[token]; ok && call.epoch == c.epoch {
		c.mu.Unlock()
		call.wg.Wait()
		return call.queueID, call.snap, call.err
	}
	call := &displayCall{epoch: c.epoch}
	call.wg.Add(1)
	c.loading[token] = call
	c.mu.Unlock()

	call.queueID, call.snap, call.err = load()

	c.mu.Lock()
	if c.loading[token] == call {
		delete(c.loading, token)
	}
	if call.err == nil && call.epoch == c.epoch {
		c.store(token, call.queueID, call.snap)
	}
	c.mu.Unlock()
	call.wg.Done()
	return call.queueID, call.snap, call.err
}

// QueueID: antrian pemilik token bila ada entri segar (tanpa menyentuh database).
func (c *DisplayCache) QueueID(token string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.byToken[token]; ok && c.now().Before(e.expires) {
		return e.queueID, true
	}
	return 0, false
}

// Invalidate membuang semua entri antrian ini (termasuk token lama yang sudah diganti).
func (c *DisplayCache) Invalidate(queueID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	for token := range c.byQueue[queueID] {
		delete(c.byToken, token)
	}
	delete(c.byQueue, queueID)
}

func (c *DisplayCache) store(token string, queueID int64, snap model.DisplaySnapshot) {
	if len(c.byToken) > 1000 {
		now := c.now()
		for t, e := range c.byToken {
			if !now.Before(e.expires) {
				delete(c.byToken, t)
				delete(c.byQueue[e.queueID], t)
			}
		}
	}
	c.byToken[token] = &displayEntry{queueID: queueID, snap: snap, expires: c.now().Add(c.ttl)}
	if c.byQueue[queueID] == nil {
		c.byQueue[queueID] = map[string]struct{}{}
	}
	c.byQueue[queueID][token] = struct{}{}
}
