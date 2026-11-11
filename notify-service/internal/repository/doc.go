// Package repository berisi akses data ke PostgreSQL (role notify_ro) untuk notify-service: query baca tiket,
// antrian, counter, dan langganan push, serta penghapusan langganan yang sudah tidak berlaku. Service memanggil
// repository, dan repository tidak pernah memanggil service.
//
// Aturan yang dipakai:
//   - Repository hanya membaca dan menulis data. Keputusan siapa yang diberi tahu ada di package service.
//   - Query memakai pgx dengan parameter posisi ($1, $2, ...). Nilai dari luar tidak pernah disambung ke SQL.
//   - Satu-satunya penulisan adalah DeleteSubscription. FirstWaiting mengembalikan nil tanpa galat bila
//     tidak ada tiket waiting.
//
// Isinya:
//   - notify_repo.go: NotifyRepository (NewNotifyRepository, Ticket, PositionOf, FirstWaiting, Subscriptions,
//     DeleteSubscription).
//   - repo_interface.go: NotifyRepositoryInterface yang menggambarkan bentuk repository.
package repository
