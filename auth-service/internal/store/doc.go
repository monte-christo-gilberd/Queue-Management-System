// Package store: akses PostgreSQL untuk auth-service lewat role database auth_rw (db/migrations/06_auth_role.sh).
//
// Isinya:
//   - store.go: Connect, UserStore (tabel users), dan TokenStore (tabel user_action_tokens).
//
// Role auth_rw hanya boleh menyentuh kedua tabel itu; tidak ada akses ke antrian, tiket, atau pembayaran.
// Yang disimpan untuk token sekali pakai hanya hash SHA-256-nya, tidak pernah token mentah.
package store
