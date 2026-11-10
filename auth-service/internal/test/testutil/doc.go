// Package testutil berisi fake di memori untuk tes auth-service: FakeStore (memenuhi auth.UserStore dan
// auth.TokenStore dengan semantik SQL: token sekali pakai, kedaluwarsa, token lama dibatalkan saat token baru dibuat),
// FakeMailer, dan NewService yang merangkai Service dengan kunci RSA tes. Hanya dipakai oleh paket tes.
package testutil
