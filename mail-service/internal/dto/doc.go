// Package dto berisi bentuk data yang masuk dan keluar lewat HTTP: body request yang dibaca handler dan
// body respons yang dikirim ke pemanggil. Tipe di sini terpisah dari model, supaya kontrak API tidak berubah
// diam-diam saat model berubah.
//
// Isinya:
//   - mail_dto.go: SendRequest (body POST /internal/v1/send), SendResponse (isi "data" saat sukses), dan
//     HealthResponse (body GET /health).
//
// Package ini tidak berisi aturan bisnis. Pemeriksaan alamat dan templat ada di service.
package dto
