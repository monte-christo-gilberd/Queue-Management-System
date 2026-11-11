// Package repository berisi akses data ke PostgreSQL untuk auth-service: setiap query SQL untuk tabel users dan
// user_action_tokens. Service memanggil repository, dan repository tidak pernah memanggil service.
//
// Aturan yang dipakai di semua repository:
//   - Repository hanya membaca dan menulis data. Aturan bisnis (password, batas per jam, login Google) ada di
//     package service.
//   - Query memakai parameter posisi ($1, $2, ...). Nilai dari luar tidak pernah disambung ke string SQL.
//   - Baris yang tidak ditemukan diterjemahkan ke apperror.ErrNotFound (sql.ErrNoRows tidak keluar dari package
//     ini). Pelanggaran UNIQUE dibaca dari kode error pgconn (23505), lalu diterjemahkan ke apperror.ErrDuplicate.
//   - Role auth_rw hanya boleh menyentuh kedua tabel itu. Untuk token sekali pakai hanya hash SHA-256-nya yang
//     disimpan, tidak pernah token mentah.
//
// Isinya:
//   - user_repo.go: UserRepository, yaitu tabel users (akun, pencarian login, login Google, waktu ganti password).
//   - token_repo.go: TokenRepository, yaitu tabel user_action_tokens (token reset password dan verifikasi email).
//   - repo_interface.go: UserRepositoryInterface dan TokenRepositoryInterface yang menggambarkan bentuk tiap
//     repository. Dipakai terutama oleh tes untuk membuat fake dalam memori.
package repository
