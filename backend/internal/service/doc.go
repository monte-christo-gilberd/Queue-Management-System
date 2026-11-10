// Package service berisi logika bisnis backend: aturan antrian, tiket, counter, riwayat, layar publik, QR,
// akun, reset password, dan verifikasi email. Handler HTTP memanggil service, lalu service memanggil repository.
//
// Aturan yang dipakai di semua service:
//   - Service tidak mengenal HTTP. Ia menerima context dan ID, lalu mengembalikan model atau apperror.
//     Handler yang menerjemahkan error itu menjadi kode HTTP.
//   - Kepemilikan dicek di service. Owner antrian memakai QueueGuard (GetQueue). Non-owner mendapat
//     apperror.ErrNotFound, bukan ErrForbidden, supaya keberadaan antrian tidak bocor.
//   - Fail closed: service yang butuh QueueGuard, AccountFinder, atau CounterTicketReleaser tetapi belum
//     dipasang akan menolak operasi (errNoQueueGuard, errNoAccountFinder, errNoTicketReleaser), bukan
//     melewatinya diam-diam. Pengecualian: TicketHolderChecker yang tidak dipasang berarti pemegang tiket
//     tidak mendapat akses baca, dan VerificationGuard yang tidak dipasang berarti tidak ada pemeriksaan.
//   - Setiap perubahan yang menyentuh beberapa tabel dijalankan dalam satu transaksi lewat TxRunner.
//   - Setelah perubahan berhasil, service mengirim event lewat EventNotifier supaya layar yang terbuka
//     (SSE) ikut diperbarui. Notifier boleh nil (mis. di tes), jadi notifyEvent diam saja bila tidak dipasang.
//
// Pola konstruktor: New...Service menerima dependency wajib. Dependency opsional dipasang lewat With...
// (mis. WithQueueGuard, WithNotifier, WithVerificationGuard). Dengan pola ini, fitur yang tidak dipasang
// di main.go tidak aktif tanpa mengubah konstruktor.
//
// Isinya:
//   - service_interface.go: interface service yang dipakai handler (QueueServiceInterface, TicketServiceInterface,
//     UserServiceInterface, EventNotifierInterface).
//   - queue_service.go: QueueService, yaitu antrian (buat, ubah, tutup, prefix, pengaturan skip) dan pengecekan langganan.
//   - ticket_service.go: TicketService, yaitu join, next, skip, recall, cancel, remove, sisip posisi, dan status antrian.
//   - counter_service.go: CounterService, yaitu CRUD counter per antrian (maksimal MaxCounters).
//   - display_service.go dan display_cache.go: DisplayService untuk layar publik beserta cache snapshot-nya.
//   - history_service.go: HistoryService, yaitu riwayat antrian yang sudah ditutup, riwayat tiket customer, dan ekspor.
//   - note_service.go: NoteService, yaitu catatan owner per tiket.
//   - qr_service.go: QRService, yaitu QR statis dan dinamis serta verifikasi token join.
//   - user_service.go: UserService, yaitu registrasi, login, profil, foto profil, dan username.
//   - password_reset_service.go dan email_verification_service.go: token sekali pakai untuk reset password dan
//     verifikasi email (token mentah hanya dikirim lewat email, di database hanya hash-nya).
//   - notifier.go: EventNotifier, nama event, dan notifyEvent.
//   - tx_runner.go: TxRunner, kontrak transaksi yang dipenuhi database.SqlxTxRunner.
//
// Package ini tidak membuka koneksi database. Koneksi dan transaksi nyata ada di package database, dan
// query ada di package repository.
package service
