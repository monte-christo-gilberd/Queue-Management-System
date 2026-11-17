// Package database berisi akses koneksi ke PostgreSQL dan pembungkus transaksi.
//
// Isinya:
//   - connect.go: membuka koneksi ke PostgreSQL memakai sqlx dan driver pgx, lalu memastikan database bisa dihubungi.
//   - tx.go: WithTx untuk menjalankan beberapa query dalam satu transaksi (commit jika berhasil, rollback jika error),
//     serta SqlxTxRunner yang dipakai service lewat interface TxRunner.
package database
