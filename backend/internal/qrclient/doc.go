// Package qrclient berisi klien HTTP untuk endpoint internal qr-service: membuat dan memverifikasi token join antrian.
//
// Isinya:
//   - client.go: Client, IssueJoinToken untuk membuat token, dan VerifyJoinToken untuk memeriksa token.
//
// Setiap panggilan membawa header X-Service-Token. Galat jaringan atau respons tak terduga diterjemahkan
// menjadi error aplikasi (apperror), sehingga detail dari qr-service tidak sampai ke pengguna.
package qrclient
