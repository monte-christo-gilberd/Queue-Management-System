// Package database berisi akses koneksi ke PostgreSQL dan pembungkus transaksi.
//
// Isinya:
//   - pool.go: PoolSettings dan ApplyPool, batas pool koneksi dari DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS, DB_CONN_MAX_LIFETIME.
//   - connect.go: membuka koneksi ke PostgreSQL memakai sqlx dan driver pgx, lalu memastikan database bisa dihubungi.
//   - tx.go: WithTx untuk menjalankan beberapa query dalam satu transaksi (commit jika berhasil, rollback jika error),
//     serta SqlxTxRunner yang dipakai service lewat interface TxRunner.
//   - database_interface.go: kontrak TxRunnerInterface untuk runner transaksi.
package database
