package model

import "time"

const (
	// Version adalah versi format token. Naikkan bila format berubah.
	Version = "v1"

	// MaxTokenLength membatasi token yang diterima agar payload raksasa ditolak lebih awal.
	MaxTokenLength = 1024

	// DefaultDynamicTTL dipakai bila pemanggil tidak menentukan TTL untuk token dinamis.
	DefaultDynamicTTL = 120 * time.Second
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
	// hanya dua nilai yang diterima
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
