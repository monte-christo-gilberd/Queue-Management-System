// Package model berisi tipe domain qr-service yang dipakai lintas lapisan: jenis token, klaim token, dan
// opsi render QR.
//
// Isinya:
//   - token.go: Type (static atau dynamic), Claims, serta konstanta format token (Version, MaxTokenLength,
//     DefaultDynamicTTL).
//   - render.go: RenderOptions, parameter untuk merender QR menjadi PNG.
//
// Package ini tidak mengimpor package lain di internal, jadi aman dipakai semua lapisan tanpa import siklik.
package model
