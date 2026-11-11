// Package service berisi logika bisnis notify-service: untuk satu event, menentukan siapa yang diberi tahu
// dan teks apa yang dikirim, lalu mengirim push ke setiap langganan. Runner memanggil service, lalu service
// memanggil repository dan pengirim push.
//
// Alur:
//   - Plan menghitung daftar Delivery (penerima dan isi pesan) dari event dan data tiket.
//   - Handle mengambil langganan setiap penerima, lalu memanggil PushSender.Send. Langganan yang sudah tidak
//     berlaku (apperror.ErrSubscriptionGone) dihapus dari database. Kegagalan satu perangkat tidak menghentikan
//     yang lain, dan galat pertama dikembalikan di akhir.
//
// Aturan yang dipakai:
//   - Interface dependensi (NotifyRepository, PushSender) dideklarasikan di sisi pemakai, yaitu di
//     notify_service.go. Dengan begitu service tidak mengimpor repository maupun pusher.
//   - Dependensi wajib dipasang lewat NewNotifyService. Tidak ada dependensi opsional, jadi tidak ada With...
//
// Isinya:
//   - notify_service.go: NotifyService (NewNotifyService, Plan, Handle), Delivery, serta interface
//     NotifyRepository dan PushSender.
//   - label.go: Label, satu-satunya tempat format nomor tiket untuk teks notifikasi.
//   - service_interface.go: NotifyServiceInterface, kontrak yang dipakai runner.
//
// Package ini tidak membuka koneksi database atau HTTP.
package service
