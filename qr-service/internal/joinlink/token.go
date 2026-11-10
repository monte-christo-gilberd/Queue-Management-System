package joinlink

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// Version adalah versi format token. Naikkan bila format berubah.
	Version = "v1"

	// MaxTokenLength membatasi token yang diterima agar payload raksasa ditolak lebih awal.
	MaxTokenLength = 1024

	// maxPayloadBytes: batas ukuran payload JSON yang sudah di-decode (512 byte).
	maxPayloadBytes = 512

	// DefaultDynamicTTL dipakai bila pemanggil tidak menentukan TTL untuk token dinamis.
	DefaultDynamicTTL = 120 * time.Second
)

// Galat yang dikembalikan Verify dan Issue. Handler menerjemahkannya ke kode HTTP lewat errors.Is.
var (
	// ErrMalformed: token tidak punya tiga bagian, panjangnya salah, atau base64 rusak.
	ErrMalformed = errors.New("malformed token")
	// ErrInvalidSignature: tanda tangan tidak cocok dengan isi token dan secret.
	ErrInvalidSignature = errors.New("invalid token signature")
	// ErrExpired: waktu sekarang sudah melewati ExpiresAt token.
	ErrExpired = errors.New("token expired")
	// ErrUnsupportedFormat: versi di depan token bukan Version yang dikenal.
	ErrUnsupportedFormat = errors.New("unsupported token version")
	// ErrInvalidPayload: isi klaim tidak valid (id antrian, jenis, JTI, atau exp).
	ErrInvalidPayload = errors.New("invalid token payload")
)

// Type adalah jenis kode QR join.
type Type string

const (
	// TypeStatic: QR tetap. Token tidak punya waktu kedaluwarsa.
	TypeStatic Type = "static"
	// TypeDynamic: QR yang berganti. Token punya waktu kedaluwarsa.
	TypeDynamic Type = "dynamic"
)

// Valid melaporkan apakah t termasuk jenis yang dikenal (static atau dynamic).
func (t Type) Valid() bool {
	// Hanya dua nilai yang diterima.
	return t == TypeStatic || t == TypeDynamic
}

// Claims adalah isi token join. Token statis tidak punya ExpiresAt.
type Claims struct {
	// QueueID: id antrian yang dituju token ini (json "qid").
	QueueID int64 `json:"qid"`
	// Type: jenis token: static atau dynamic (json "typ").
	Type Type `json:"typ"`
	// JTI: id unik token, dibuat acak saat Issue (json "jti").
	JTI string `json:"jti"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix (json "exp"). Nol untuk token statis dan dihilangkan dari JSON.
	ExpiresAt int64 `json:"exp,omitempty"`
}

// Signer membuat dan memverifikasi token join bertanda tangan HMAC-SHA256.
// Token tidak menyimpan data rahasia; tanda tangan mencegah pemalsuan.
type Signer struct {
	// secret: kunci HMAC untuk menandatangani dan memverifikasi token.
	secret []byte
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
}

// New membuat Signer. Secret minimal 32 byte agar tidak mudah ditebak.
func New(secret string) (*Signer, error) {
	// Secret kurang dari 32 byte terlalu mudah ditebak: tolak.
	if len(secret) < 32 {
		return nil, fmt.Errorf("join link secret must be at least 32 bytes")
	}
	// Secret cukup panjang: buat Signer dengan time.Now sebagai sumber waktu.
	return &Signer{secret: []byte(secret), now: time.Now}, nil
}

// Issue membuat token untuk satu antrian.
// Token statis tidak punya exp; token dinamis memakai ttl (default DefaultDynamicTTL).
func (s *Signer) Issue(queueID int64, typ Type, ttl time.Duration) (string, Claims, error) {
	// Id antrian tidak positif: tolak sebagai payload tidak valid.
	if queueID <= 0 {
		return "", Claims{}, ErrInvalidPayload
	}
	// Jenis tidak dikenal: tolak sebagai payload tidak valid.
	if !typ.Valid() {
		return "", Claims{}, ErrInvalidPayload
	}

	// Buat id unik acak untuk token ini.
	jti, err := randomID()
	// Gagal membuat id acak: bungkus galatnya dan kembalikan.
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate jti: %w", err)
	}

	// Susun klaim dasar: antrian, jenis, dan id unik.
	claims := Claims{
		QueueID: queueID,
		Type:    typ,
		JTI:     jti,
	}
	// Token dinamis: hitung waktu kedaluwarsa dari sekarang ditambah TTL.
	if typ == TypeDynamic {
		// TTL tidak diisi (nol atau negatif): pakai bawaan.
		if ttl <= 0 {
			ttl = DefaultDynamicTTL
		}
		// Simpan waktu kedaluwarsa dalam detik Unix.
		claims.ExpiresAt = s.now().Add(ttl).Unix()
	}

	// Ubah klaim ke JSON sebagai payload token.
	payload, err := json.Marshal(claims)
	// Gagal membuat JSON: bungkus galatnya dan kembalikan.
	if err != nil {
		return "", Claims{}, fmt.Errorf("marshal claims: %w", err)
	}
	// Payload terlalu besar: tolak agar verifikasi nanti tetap cepat.
	if len(payload) > maxPayloadBytes {
		return "", Claims{}, ErrInvalidPayload
	}

	// Kodekan payload ke base64url tanpa padding, lalu gabungkan dengan versi.
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := Version + "." + payloadB64
	// Tandatangani bagian versi.payload, lalu tambahkan tanda tangan sebagai bagian ketiga.
	token := signingInput + "." + s.sign(signingInput)

	return token, claims, nil
}

// Verify memeriksa format, tanda tangan, dan masa berlaku token.
func (s *Signer) Verify(token string) (Claims, error) {
	// Token kosong atau terlalu panjang: tolak sebagai format salah.
	if token == "" || len(token) > MaxTokenLength {
		return Claims{}, ErrMalformed
	}

	// Pecah token menjadi tiga bagian: versi, payload, dan tanda tangan.
	parts := strings.Split(token, ".")
	// Jumlah bagian bukan tiga: format salah.
	if len(parts) != 3 {
		return Claims{}, ErrMalformed
	}
	// Versi tidak dikenal: tolak sebagai versi tidak didukung.
	if parts[0] != Version {
		return Claims{}, ErrUnsupportedFormat
	}

	// Decode payload dari base64url. Gagal decode, payload kosong, atau terlalu besar: tolak.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(payload) == 0 || len(payload) > maxPayloadBytes {
		return Claims{}, ErrMalformed
	}

	// Hitung ulang tanda tangan dari versi.payload, lalu bandingkan dalam waktu tetap.
	expected := s.sign(parts[0] + "." + parts[1])
	// Tanda tangan tidak cocok: token dipalsukan atau diubah.
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrInvalidSignature
	}

	// Tanda tangan benar: baca klaim dari payload JSON.
	var claims Claims
	// JSON tidak bisa dibaca: format salah.
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrMalformed
	}
	// Id antrian tidak positif, jenis tidak dikenal, atau JTI kosong: isi payload tidak valid.
	if claims.QueueID <= 0 || !claims.Type.Valid() || claims.JTI == "" {
		return Claims{}, ErrInvalidPayload
	}
	// Token dinamis tanpa waktu kedaluwarsa: tidak boleh ada, jadi tolak.
	if claims.Type == TypeDynamic && claims.ExpiresAt == 0 {
		return Claims{}, ErrInvalidPayload
	}
	// Waktu sekarang sudah sampai atau melewati kedaluwarsa: token tidak berlaku lagi.
	if claims.ExpiresAt > 0 && s.now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrExpired
	}

	// Semua pemeriksaan lolos: kembalikan klaim.
	return claims, nil
}

// sign menghitung HMAC-SHA256 dari signingInput dengan secret, lalu mengodekannya ke base64url tanpa padding.
func (s *Signer) sign(signingInput string) string {
	// Buat HMAC dengan SHA-256 dan secret milik Signer.
	mac := hmac.New(sha256.New, s.secret)
	// Masukkan teks yang ditandatangani ke HMAC.
	mac.Write([]byte(signingInput))
	// Kodekan hasil tanda tangan ke base64url tanpa padding agar aman di URL.
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// randomID membuat id acak 16 byte dalam base64url tanpa padding. Dipakai sebagai JTI.
func randomID() (string, error) {
	// Siapkan buffer 16 byte acak.
	buf := make([]byte, 16)
	// Isi dengan byte acak dari crypto/rand; galat dikembalikan ke pemanggil.
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Kodekan ke base64url tanpa padding.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
