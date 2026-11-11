// Package httpserver membungkus http.Server dengan timeout dasar agar koneksi lambat tidak menggantung.
//
// Isinya:
//   - server.go: NewServer, pembuat http.Server dengan timeout baca, tulis, dan koneksi diam.
package httpserver
