// Package apperror berisi error bersama yang dipakai lintas layer mail-service (mailer, mailtemplate, service,
// handler).
//
// Setiap error adalah variabel sentinel: service dan mailer mengembalikannya, dan handler mencocokkannya dengan
// errors.Is untuk memilih status HTTP dan pesan yang dikirim ke pemanggil. Karena itu, error yang dibungkus dengan
// fmt.Errorf("...: %w", ...) tetap bisa dikenali. Worker outbox juga memakai errors.Is(err, ErrPermanent) untuk
// membedakan kegagalan SMTP permanen dari yang sementara.
//
// Isinya:
//   - errors.go: semua sentinel error, dikelompokkan menurut penyebabnya (templat, input, pengiriman).
package apperror
