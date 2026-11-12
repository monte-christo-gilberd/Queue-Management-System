// Package database membuka koneksi PostgreSQL untuk auth-service lewat role database auth_rw
// (db/migrations/06_auth_role.sh). Package ini hanya membuka koneksi; query ada di package repository.
//
// Isinya:
//   - pool.go: PoolSettings dan ApplyPool, batas pool koneksi dari DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS, DB_CONN_MAX_LIFETIME.
//   - connect.go: Connect, yang membuka koneksi dengan driver pgx lalu memastikan database terjangkau (Ping, 5 detik).
package database
