// Package store berisi query baca ke PostgreSQL (role notify_ro) dan penghapusan langganan yang sudah tidak berlaku.
//
// Semua query hanya membaca tabel tickets, queues, counters, dan push_subscriptions. Satu-satunya penulisan
// adalah DeleteSubscription.
//
// Isinya:
//   - store.go: PG, implementasi notify.Store (New, Ticket, PositionOf, FirstWaiting, Subscriptions, DeleteSubscription).
//
// Package ini tidak mengirim notifikasi. Keputusan siapa yang diberi tahu ada di package notify.
package store
