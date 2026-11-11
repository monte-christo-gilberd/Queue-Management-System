// Package model berisi bentuk data yang dipakai auth-service antarlapisan.
//
// Isinya:
//   - user.go: User, yaitu baris tabel users.
//   - google_identity.go: GoogleIdentity, yaitu identitas dari ID token Google yang sudah diperiksa. Ada di sini
//     supaya package google tidak perlu mengimpor service.
package model
