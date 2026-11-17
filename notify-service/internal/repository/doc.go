// Package repository berisi akses data ke PostgreSQL (role notify_ro) untuk notify-service: query baca tiket,
// antrian, counter, dan langganan push (Web Push di push_subscriptions, token FCM di push_devices), serta penghapusan langganan yang sudah tidak berlaku. Service memanggil
// repository, dan repository tidak pernah memanggil service.
//
// Aturan yang dipakai:
//   - Repository hanya membaca dan menulis data. Keputusan siapa yang diberi tahu ada di package service.
//   - Query memakai sqlx (driver pgx) dengan parameter posisi ($1, $2, ...). Nilai dari luar tidak pernah disambung ke SQL.
//   - Satu-satunya penulisan adalah DeleteSubscription (push_subscriptions menurut endpoint, atau push_devices
//     menurut token, sesuai jenis langganan). push_devices hanya dibaca bila EnableDevices dipanggil (FCM aktif);
//     galat membacanya tidak membatalkan Web Push. FirstWaiting mengembalikan nil tanpa galat bila
//     tidak ada tiket waiting.
//
// Isinya:
//   - notify_repo.go: NotifyRepository (NewNotifyRepository, EnableDevices, Ticket, PositionOf, FirstWaiting, Subscriptions,
//     DeleteSubscription).
//   - repo_interface.go: NotifyRepositoryInterface yang menggambarkan bentuk repository.
package repository
