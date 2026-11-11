// Package runner menjalankan siklus hidup pemrosesan event notify-service: mendengarkan event, memproses tiap
// event di goroutine sendiri, menyambung ulang saat koneksi putus, dan menunggu proses kirim saat shutdown.
//
// Waktu yang dipakai:
//   - 20 detik batas waktu per event.
//   - 3 detik jeda sebelum menyambung ulang listener.
//   - 10 detik batas menunggu proses kirim yang masih berjalan saat shutdown.
//
// Isinya:
//   - runner.go: Runner (New, Run), EventHandler, dan ListenFunc.
//
// Package ini tidak membuka koneksi sendiri. Pendengar event dan pemroses event dipasang dari main.
package runner
