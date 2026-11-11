// Package pusher mengirim Web Push ke browser lewat protokol VAPID, dengan retry untuk kegagalan sementara.
// Package ini adalah klien keluar ke layanan eksternal (push service milik browser), bukan akses database.
//
// Isinya:
//   - pusher.go: Sender (New, Send, once). Respons 404/410 diterjemahkan ke apperror.ErrSubscriptionGone,
//     sedangkan 429, 5xx, dan galat jaringan ke apperror.ErrTemporaryPush.
//   - retry.go: Retry, yaitu percobaan ulang dengan jeda berlipat. Hanya apperror.ErrTemporaryPush yang diulang.
//
// Package ini tidak tahu isi notifikasi atau siapa penerimanya. Itu urusan package service.
package pusher
