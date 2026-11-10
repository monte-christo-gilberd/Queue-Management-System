// Package google memverifikasi ID token Google (Sign-In) di sisi server.
//
// Isinya:
//   - verifier.go: Verifier, yang mengambil kunci publik Google (JWKS), memeriksa tanda tangan RS256, lalu issuer,
//     audience (daftar GOOGLE_CLIENT_IDS), masa berlaku, dan email_verified.
//
// Klien tidak dipercaya: email dan sub hanya dipakai setelah token lolos semua pemeriksaan di sini.
package google
