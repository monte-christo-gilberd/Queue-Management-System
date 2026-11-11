package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// maxLinkAttempts: salah password yang diizinkan per link token sebelum token dibakar.
const maxLinkAttempts = 5

// linkKey: kunci penghitung percobaan, berupa hash link token (token mentah tidak disimpan di memori lebih lama
// dari yang perlu).
func linkKey(token string) string {
	// Hitung SHA-256 token agar yang tersimpan di memori hanya hash, bukan token mentah.
	sum := sha256.Sum256([]byte(token))
	// Kembalikan hash dalam bentuk hex sebagai kunci peta.
	return hex.EncodeToString(sum[:])
}

// linkAttempts menghitung salah password per link token di memori. Link token hanya hidup 5 menit, jadi entri
// dibersihkan saat lewat batas itu. Cukup untuk satu instance (auth-service berjalan satu instance).
type linkAttempts struct {
	// mu: mengunci byKey karena diakses dari banyak request sekaligus.
	mu sync.Mutex
	// byKey: jumlah salah password dan kapan entri boleh dibuang.
	byKey map[string]*attemptEntry
}

// attemptEntry: catatan satu link token (salah password, sudah dibakar, kapan boleh dibuang).
type attemptEntry struct {
	// fails: jumlah salah password untuk link token ini.
	fails int
	// burned: true bila link token sudah dipakai atau dibakar dan tidak boleh dipakai lagi.
	burned bool
	// expires: waktu entri boleh dibuang (sedikit setelah link token kedaluwarsa).
	expires time.Time
}

// newLinkAttempts membuat penghitung kosong; satu per Service.
func newLinkAttempts() *linkAttempts {
	// peta kosong siap dipakai
	return &linkAttempts{byKey: map[string]*attemptEntry{}}
}

// prune membuang entri yang sudah lewat masa hidup link token. Dipanggil dengan mu terkunci.
func (l *linkAttempts) prune(now time.Time) {
	// Periksa semua entri untuk mencari yang sudah lewat masa hidupnya.
	for k, e := range l.byKey {
		// Entri sudah kedaluwarsa: link tokennya juga sudah tidak berlaku.
		if now.After(e.expires) {
			// Buang entri supaya peta tidak tumbuh tanpa batas.
			delete(l.byKey, k)
		}
	}
}

// entry mengambil catatan untuk key, atau membuatnya bila belum ada. Dipanggil dengan mu terkunci.
func (l *linkAttempts) entry(
	key string,
	now time.Time,
) *attemptEntry {
	// Cari catatan yang sudah ada untuk token ini.
	e, ok := l.byKey[key]
	// Belum ada catatan: ini percobaan pertama untuk token ini.
	if !ok {
		// Buat catatan baru; masa hidupnya sedikit lebih lama dari link token agar tidak dibuang lebih dulu.
		e = &attemptEntry{expires: now.Add(linkTokenTTL + time.Minute)}
		// Simpan catatan di peta.
		l.byKey[key] = e
	}
	// Kembalikan catatan (yang lama atau yang baru dibuat).
	return e
}

// exhausted: true bila token sudah dibakar atau terlalu sering salah password.
func (l *linkAttempts) exhausted(
	key string,
	now time.Time,
) bool {
	// Kunci mutex karena peta diakses dari banyak request sekaligus.
	l.mu.Lock()
	// Buka kunci saat fungsi selesai.
	defer l.mu.Unlock()
	// Buang entri kedaluwarsa lebih dulu agar hasilnya akurat.
	l.prune(now)
	// Ambil catatan token; tanpa catatan berarti belum pernah salah.
	e, ok := l.byKey[key]
	// Habis bila token sudah dibakar atau salah password mencapai batas.
	return ok && (e.burned || e.fails >= maxLinkAttempts)
}

// fail mencatat satu salah password.
func (l *linkAttempts) fail(
	key string,
	now time.Time,
) {
	// Kunci mutex karena peta diakses dari banyak request sekaligus.
	l.mu.Lock()
	// Buka kunci saat fungsi selesai.
	defer l.mu.Unlock()
	// Tambah hitungan salah password pada catatan token.
	l.entry(key, now).fails++
}

// burn menandai token terpakai (sekali pakai).
func (l *linkAttempts) burn(
	key string,
	now time.Time,
) {
	// Kunci mutex karena peta diakses dari banyak request sekaligus.
	l.mu.Lock()
	// Buka kunci saat fungsi selesai.
	defer l.mu.Unlock()
	// Tandai token sudah dibakar supaya tidak bisa dipakai lagi.
	l.entry(key, now).burned = true
}
