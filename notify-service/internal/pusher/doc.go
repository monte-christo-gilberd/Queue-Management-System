// Package pusher mengirim Web Push ke browser lewat protokol VAPID, dengan retry untuk kegagalan sementara, dan
// memilih kanal pengiriman (Web Push atau FCM) per langganan. Package ini adalah klien keluar ke layanan eksternal
// (push service milik browser), bukan akses database.
//
// Isinya:
//   - pusher.go: Sender (New, Send, once). Respons 404/410 diterjemahkan ke apperror.ErrSubscriptionGone,
//     sedangkan 429, 5xx, dan galat jaringan ke apperror.ErrTemporaryPush.
//   - retry.go: Retry, yaitu percobaan ulang dengan jeda berlipat. Hanya apperror.ErrTemporaryPush yang diulang.
//     Dipakai juga oleh package fcm.
//   - dispatch.go: Dispatcher (NewDispatcher, Send) dan ChannelSender. Meneruskan langganan berjenis
//     model.KindFCM ke pengirim FCM (package fcm) dan sisanya ke Sender Web Push. Pengirim FCM boleh nil (kredensial
//     kosong); langganan FCM lalu ditolak dengan ErrChannelDisabled, dan Web Push tetap jalan.
//
// Package ini tidak tahu isi notifikasi atau siapa penerimanya. Itu urusan package service.
package pusher
