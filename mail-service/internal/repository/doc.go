// Package repository berisi akses data ke PostgreSQL untuk outbox email: klaim baris, tandai terkirim, jadwal
// ulang, tandai gagal, dan mendengarkan NOTIFY. Service memanggil repository, dan repository tidak pernah
// memanggil service.
//
// Aturan yang dipakai:
//   - Repository hanya membaca dan menulis data. Aturan bisnis (laju, batas percobaan, backoff) ada di service.
//   - Query memakai parameter posisi ($1, $2, ...). Nilai dari luar tidak pernah disambung ke string SQL.
//   - Kolom last_error dipotong ke 500 byte sebelum disimpan.
//
// Isinya:
//   - outbox_repo.go: OutboxRepository (NewOutboxRepository, Claim, MarkSent, MarkRetry, MarkFailed) dan claimSQL.
//   - outbox_listener.go: Listen dan listenOnce, yaitu LISTEN mail_outbox yang membangunkan worker (koneksi pgx khusus, karena database/sql tidak punya WaitForNotification).
//   - database.go: ConnectWithRetry, yaitu membuka pool koneksi (sqlx dengan driver pgx) dan mengulang sampai database terjangkau.
//   - repo_interface.go: OutboxRepositoryInterface, kontrak yang dipakai terutama oleh tes.
package repository
