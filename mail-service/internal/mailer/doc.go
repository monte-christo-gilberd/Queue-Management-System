// Package mailer berisi klien SMTP: menyusun pesan multipart (teks dan HTML), mengatur batas waktu dan mode TLS,
// serta mengulang kegagalan sementara dengan backoff. Dipakai oleh service (kirim langsung dan worker outbox),
// sehingga inilah satu-satunya jalur keluar email di QMS.
//
// Isinya:
//   - mailer.go: Mailer (New, WithRetry, SetSleep, Send), sendOnce (satu percakapan SMTP), build (menyusun MIME),
//     dan classify (memisahkan galat permanen, apperror.ErrPermanent, dari yang sementara).
//
// Package ini tidak tahu asal email (API atau outbox) dan tidak mengisi templat. Itu urusan service dan
// package mailtemplate.
package mailer
