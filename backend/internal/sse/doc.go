// Package sse mengirim event real-time ke browser dan aplikasi lewat Server-Sent Events (SSE): koneksi HTTP satu arah
// dari server ke klien, di mana server terus menulis baris "data: ..." sampai koneksi ditutup.
//
// Istilah yang dipakai:
//   - Room: kumpulan koneksi yang menonton satu antrian, dikunci dengan queue_id. Layar publik punya room sendiri
//     (DisplayRoom) yang hanya menerima sinyal tanpa data tiket.
//   - Heartbeat: baris komentar ": ping" berkala agar koneksi yang diam tidak diputus proxy atau load balancer.
//   - Broadcast: menyebarkan satu pesan ke semua klien di satu room.
//
// Isinya:
//   - hub.go: Hub, yang menyimpan room dan menyebarkan pesan. Satu goroutine Run() memproses register,
//     unregister, dan broadcast agar map room tidak diakses bersamaan.
//   - client.go: Client, satu koneksi SSE yang menulis event ke browser (Serve) dan mengirim heartbeat.
//   - message.go: nama-nama event dan Event, bentuk JSON yang dikirim ke klien.
//
// Package ini tidak tahu cara antrian diubah. Service memanggil Hub.Notify setelah ada perubahan, lalu event
// disebarkan ke klien yang sedang terhubung.
package sse
