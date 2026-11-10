// Package push mengirim Web Push ke browser lewat protokol VAPID, dengan retry untuk kegagalan sementara.
//
// Isinya:
//   - push.go: Sender (New, Send, once) dan ErrGone (langganan sudah tidak berlaku).
//   - retry.go: ErrTemporary (kegagalan yang layak dicoba ulang) dan Retry (jeda berlipat).
//
// Package ini tidak tahu isi notifikasi atau siapa penerimanya. Itu urusan package notify.
package push
