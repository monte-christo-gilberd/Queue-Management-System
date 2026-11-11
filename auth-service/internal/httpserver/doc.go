// Package httpserver membungkus http.Server dengan batas waktu dasar (header 10 detik, baca 30 detik, tulis 30 detik,
// idle 60 detik) agar koneksi yang macet tidak menggantung. Penghentian server dengan rapi dilakukan di
// cmd/auth-service/main.go.
//
// Isinya:
//   - server.go: New, yang membuat http.Server dengan batas waktu di atas.
package httpserver
