// Package dto berisi bentuk data yang dikirim keluar dari notify-service.
//
// Isinya:
//   - push_dto.go: Message, isi satu notifikasi push (title, body, url). Web Push mengirimnya sebagai JSON ke
//     browser; FCM memetakannya ke notification.title/body dan data.url.
//
// Package ini tidak punya dependensi ke package lain di service ini.
package dto
