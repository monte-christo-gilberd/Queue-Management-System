// Package model berisi bentuk data inti mail-service: pesan email, hasil render templat, dan baris outbox.
//
// Aturan yang dipakai di semua model:
//   - Model hanya menyimpan data. Tidak ada logika bisnis dan tidak ada akses jaringan atau database.
//   - Model tidak dikirim langsung ke pemanggil API. Bentuk JSON respons ada di package dto.
//
// Isinya:
//   - message.go: Message, yaitu satu email yang siap dikirim lewat SMTP.
//   - rendered.go: Rendered, yaitu subjek, teks, dan HTML hasil render templat.
//   - outbox.go: OutboxItem (baris outbox yang diklaim) dan OutboxPayload (isi kolom payload).
package model
