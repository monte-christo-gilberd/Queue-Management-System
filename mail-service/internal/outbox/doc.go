// Package outbox berisi worker yang mengirim email dari tabel notification_outbox. Trigger database menulis baris
// dalam transaksi yang sama dengan perubahan tiket, sehingga email tidak hilang walau layanan ini mati.
//
// Isinya:
//   - outbox.go: Worker (NewWorker, Run, Wake, drain, deliver), Config, Item, Store, dan Sender (kontrak),
//     serta compose yang menyusun email dari baris outbox.
//   - pgstore.go: PG, implementasi Store di atas PostgreSQL, dan Listen yang membangunkan worker saat NOTIFY tiba.
//
// Pengiriman bersifat at-least-once: bila proses mati tepat setelah SMTP menerima pesan dan sebelum baris
// ditandai terkirim, email bisa terkirim dua kali setelah masa sewa (lease) habis.
package outbox
