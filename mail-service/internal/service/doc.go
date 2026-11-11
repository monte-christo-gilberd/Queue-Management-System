// Package service berisi logika bisnis mail-service: kirim email langsung (periksa penerima, isi templat, batasi
// pengiriman bersamaan) dan worker outbox (klaim baris, susun email, kirim sesuai laju, catat hasil). Handler HTTP
// dan main memanggil service, lalu service memanggil mailer, mailtemplate, dan repository.
//
// Aturan yang dipakai di semua service:
//   - Service tidak mengenal HTTP. Ia menerima context dan nilai biasa, lalu mengembalikan apperror.
//     Handler yang menerjemahkan error itu menjadi kode HTTP.
//   - Dependency dinyatakan sebagai interface di sisi pemakai (Sender, TemplateRenderer, OutboxStore), jadi tes
//     cukup memakai tiruan dan tidak perlu SMTP atau database.
//   - Kegagalan SMTP yang tidak ada gunanya diulang dikenali lewat errors.Is(err, apperror.ErrPermanent).
//
// Isinya:
//   - mail_service.go: MailService (Send, SendTest), semafor 16 pengiriman bersamaan, batas waktu 45 detik, serta
//     interface Sender dan TemplateRenderer.
//   - outbox_service.go: OutboxService (NewOutboxService, Run, Wake, drain, deliver, fail, backoff, compose),
//     OutboxConfig, interface OutboxStore, dan Label.
//   - service_interface.go: MailServiceInterface dan OutboxServiceInterface, kontrak yang dipakai handler dan main.
//
// Package ini tidak membuka koneksi SMTP atau database. Itu urusan package mailer dan repository.
package service
