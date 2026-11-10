// Package repository berisi akses data ke PostgreSQL: setiap query SQL untuk tabel antrian, tiket, counter,
// user, riwayat, layar publik, catatan, token, dan foto profil. Service memanggil repository, dan repository
// tidak pernah memanggil service.
//
// Aturan yang dipakai di semua repository:
//   - Repository hanya membaca dan menulis data. Aturan bisnis (siapa boleh melakukan apa, urutan antrian,
//     batas jumlah) ada di package service. Pengecekan kepemilikan juga ada di service, kecuali yang
//     dinyatakan di komentar repository (mis. query riwayat hanya mengembalikan milik owner).
//   - Query memakai sqlx dengan parameter bernama posisi ($1, $2, ...). Nilai dari luar tidak pernah
//     disambung ke string SQL, jadi tidak ada SQL injection.
//   - Metode yang menerima *sqlx.Tx dipakai service di dalam transaksi (lihat database.WithTx). Metode
//     tanpa tx membaca langsung dari *sqlx.DB.
//   - Baris yang tidak ditemukan diterjemahkan ke apperror.ErrNotFound (sql.ErrNoRows tidak keluar dari
//     package ini). Pelanggaran UNIQUE dibaca dari kode error pgconn (23505), lalu diterjemahkan ke
//     apperror.ErrDuplicate, atau ErrDuplicateCounterName untuk nama counter yang bentrok.
//
// Isinya:
//   - queue_repo.go: QueueRepository, yaitu antrian, pengaturan skip, QR, dan counter default.
//   - ticket_repo.go: TicketRepository, yaitu tiket, pengurutan sort_key, dan status tiket.
//   - counter_repo.go: CounterRepository, yaitu counter per antrian dan nomor yang sedang dilayani.
//   - user_repo.go: UserRepository, yaitu akun dan pencarian login.
//   - owner_repo.go: SubscriptionChecker, yaitu cek langganan owner aktif.
//   - history_repo.go: HistoryRepository, yaitu riwayat antrian yang sudah ditutup dan riwayat tiket customer.
//   - display_repo.go: DisplayRepository, yaitu konfigurasi layar publik.
//   - note_repo.go: NoteRepository, yaitu catatan per tiket.
//   - password_reset_repo.go dan email_verification_repo.go: token sekali pakai untuk reset password dan
//     verifikasi email.
//   - avatar_repo.go: AvatarRepository, yaitu foto profil yang disimpan di database.
//   - push_repo.go: PushRepository, yaitu langganan push notifikasi.
//   - repo_interface.go: kumpulan interface *RepositoryInterface yang menggambarkan bentuk tiap repository.
//     Dipakai terutama oleh tes untuk membuat fake dalam memori.
package repository
