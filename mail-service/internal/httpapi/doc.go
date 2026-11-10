// Package httpapi berisi API HTTP internal mail-service. Hanya dijangkau dari jaringan internal (tidak ada rute
// di nginx), dan setiap pemanggilan ke endpoint kirim wajib membawa X-Service-Token, pola yang sama dengan qr-service.
//
// Isinya:
//   - httpapi.go: API (New, Handler, auth, send) dan SendRequest, serta pembantu writeJSON dan writeErr.
//
// Package ini tidak menyusun isi email sendiri. Isi dibentuk oleh package mailer dari nama templat dan data
// yang dikirim pemanggil, lalu dikirim lewat Sender.
package httpapi
