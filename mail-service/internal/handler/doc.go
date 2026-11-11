// Package handler berisi handler HTTP (net/http) yang menerima request, memeriksa input, memanggil service,
// lalu menerjemahkan hasilnya menjadi respons JSON.
//
// Pola yang dipakai di semua handler:
//   - Body dibaca dengan batas 64 KB dan decoder yang menolak field tak dikenal. Body yang salah dibalas
//     400 BAD_REQUEST, dan yang terlalu besar 413 PAYLOAD_TOO_LARGE.
//   - Error dari service diterjemahkan lewat sentinel error di apperror (errors.Is) menjadi kode HTTP dan
//     kode error yang tetap, misalnya 400 UNKNOWN_TEMPLATE, 503 BUSY, atau 502 SMTP_FAILED.
//   - Error yang tidak dikenal dibalas 500 INTERNAL_ERROR tanpa detail, supaya pesan teknis (termasuk detail SMTP)
//     tidak bocor ke pemanggil.
//   - Respons sukses dibungkus di field "data" lewat respondData, dan respons error di field "error" lewat
//     respondError. Health check adalah pengecualian: bodynya {"status":"ok"} tanpa pembungkus.
//
// Isinya:
//   - response.go: respondJSON, respondData, dan respondError, pembantu untuk bentuk respons JSON.
//   - handler_interface.go: interface handler yang dipakai router.
//   - mail_handler.go: MailHandler (Send) dan interface MailService yang dibutuhkannya.
//   - system_handler.go: SystemHandler (Health dan NotFound).
//
// Handler hanya mengatur HTTP. Pemeriksaan penerima, pengisian templat, dan batas pengiriman bersamaan ada di
// package service.
package handler
