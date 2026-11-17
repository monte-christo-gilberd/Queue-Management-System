// Package dto berisi bentuk data yang masuk dan keluar lewat HTTP dan SSE: body request yang dibaca handler,
// dan response JSON yang dikirim ke klien. Tipe di sini terpisah dari model (tabel database), supaya perubahan
// kolom tidak langsung mengubah kontrak API.
//
// Pola yang dipakai:
//   - Struct request (mis. CreateQueueRequest, LoginRequest) dipetakan dari body JSON oleh handler.
//   - Struct response (mis. QueueResponse, TicketResponse) dibuat dari model lewat fungsi New...
//     (mis. NewQueueResponse, NewTicketResponse). Fungsi New... hanya menyalin dan merapikan nilai,
//     tidak mengakses database.
//   - Payload event real-time untuk tiket (TicketEvent) dibuat dengan NewTicketEvent, dengan cara yang sama
//     seperti response lainnya.
//   - Bila model memakai sql.Null*, dto mengubahnya menjadi pointer (nullTimePtr, counterNamePtr), supaya
//     nilai kosong terlihat sebagai null di JSON, bukan zero value.
//
// Isinya per kelompok fitur:
//   - queue_dto.go: antrian, QR join, pengaturan skip, dan NormalizeTicketPrefix (merapikan prefix nomor).
//   - ticket_dto.go: tiket, status antrian, dan tiket milik user.
//   - counter_dto.go: counter beserta nomor yang sedang dilayani.
//   - display_dto.go: konfigurasi dan snapshot layar publik.
//   - history_dto.go: riwayat antrian dan riwayat tiket customer, termasuk cursor halaman.
//   - note_dto.go: catatan tiket.
//   - user_dto.go: registrasi, login, profil, dan reset password.
//   - push_dto.go: langganan Web Push dan token FCM perangkat Android (FCMRegisterRequest, FCMUnregisterRequest).
//
// Package ini tidak berisi aturan bisnis. Validasi aturan (mis. panjang nama, batas jumlah) ada di service,
// dan dto hanya menyiapkan bentuk datanya.
package dto
