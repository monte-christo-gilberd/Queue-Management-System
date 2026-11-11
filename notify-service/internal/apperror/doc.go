// Package apperror berisi error bersama yang dipakai lintas layer notify-service (pusher dan service).
//
// Setiap error adalah variabel sentinel: pusher mengembalikannya, dan service mencocokkannya dengan
// errors.Is untuk memilih tindakan (hapus langganan, atau coba ulang). Karena itu, error yang dibungkus
// dengan fmt.Errorf("...: %w", ...) tetap bisa dikenali.
//
// Isinya:
//   - apperror.go: ErrSubscriptionGone (langganan sudah tidak berlaku) dan ErrTemporaryPush (kegagalan
//     sementara yang layak dicoba ulang).
//
// Package ini tidak punya dependensi ke package lain di service ini, sehingga service tidak perlu
// mengimpor pusher.
package apperror
