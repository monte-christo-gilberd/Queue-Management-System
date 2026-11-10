// Package joinlink membuat dan memverifikasi token join QR: string bertanda tangan HMAC-SHA256 yang berisi
// id antrian, jenis QR, id unik, dan (untuk QR dinamis) waktu kedaluwarsa.
//
// Format token: "v1.<payload base64url>.<signature base64url>". Payload berisi JSON Claims, dan tanda tangan
// mencegah token dipalsukan atau diubah isinya. Token tidak menyimpan data rahasia.
//
// Isinya:
//   - token.go: Signer (New, Issue, Verify), Claims, Type, dan galat yang dikenali (ErrMalformed, ErrExpired, dll.).
//
// Package ini tidak tahu apakah token masih cocok dengan database. Backend yang mencocokkan token statis
// dengan queue_settings.
package joinlink
