// Package testutil berisi fake di memori untuk tes auth-service: FakeStore (memenuhi service.UserRepository dan
// service.TokenRepository dengan semantik SQL: token sekali pakai, kedaluwarsa, token lama dibatalkan saat token baru
// dibuat), FakeMailer, FakeGoogle, NewService yang merangkai Service dengan kunci RSA tes, dan NewEngine yang
// merangkai router HTTP seperti di main. Hanya dipakai oleh paket tes.
//
// Isinya:
//   - testutil.go: semua fake dan pembantu rakitan di atas.
package testutil
