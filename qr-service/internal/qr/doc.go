// Package qr merender kode QR menjadi PNG. Dipakai endpoint /internal/v1/qr/render untuk QR umum
// (mis. QR pembayaran), dan tidak mengetahui apa pun tentang antrian.
//
// Isinya:
//   - render.go: Options dan Render, serta pembantu parseRecoveryLevel dan parseHexColor.
package qr
