// Package mailer mengirim email lewat SMTP: menyusun pesan multipart (teks dan HTML), mengatur batas waktu dan
// mode TLS, serta mengulang kegagalan sementara dengan backoff. Dipakai oleh httpapi (kirim langsung) dan
// outbox (worker), sehingga inilah satu-satunya jalur keluar email di QMS.
//
// Isinya:
//   - mailer.go: Mailer, Message, ErrPermanent, dan Send (dengan retry), sendOnce (satu percakapan SMTP), build
//     (menyusun MIME), serta classify (memisahkan galat permanen dari yang sementara).
//   - templates.go: Render, Has, Rendered, dan galat templat. Templat ada di folder templates/.
//
// Package ini tidak tahu asal email (API atau outbox). Itu urusan pemanggilnya.
package mailer
