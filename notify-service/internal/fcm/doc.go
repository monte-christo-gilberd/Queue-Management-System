// Package fcm mengirim notifikasi push ke perangkat Android lewat Firebase Cloud Messaging HTTP v1.
//
// Isinya:
//   - token.go: Credentials (file JSON service account) dan tokenSource, yang menukar JWT RS256 bertanda tangan
//     service account dengan access token OAuth2 lalu menyimpannya sampai hampir kedaluwarsa. Hanya memakai
//     library standar, tanpa SDK Firebase.
//   - sender.go: Sender.Send mengirim satu pesan ke satu token. Hasil dipetakan ke galat yang sama dengan Web Push:
//     UNREGISTERED atau token tidak sah menjadi apperror.ErrSubscriptionGone (token dihapus dari push_devices),
//     sedangkan 401, 429, 5xx, UNAVAILABLE, dan INTERNAL menjadi apperror.ErrTemporaryPush (dicoba ulang lewat
//     pusher.Retry).
package fcm
