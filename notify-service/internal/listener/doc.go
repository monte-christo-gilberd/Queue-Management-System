// Package listener mengurai payload NOTIFY dari trigger tickets_notify di database dan mendengarkannya
// lewat perintah LISTEN queue_events. Package ini adalah IO masuk dari PostgreSQL, bukan repository.
//
// Isinya:
//   - listener.go: Parse (mengurai dan memeriksa payload JSON menjadi model.Event) dan Listen (koneksi
//     LISTEN khusus lewat pgx.Connect, bukan dari pool, yang berjalan sampai koneksi putus atau context selesai).
//
// Package ini tidak tahu siapa yang harus diberi tahu. Itu urusan package service.
package listener
