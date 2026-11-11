// Package model berisi tipe data bersama notify-service: Ticket, Subscription, dan Event beserta konstanta
// jenis (Kind*) dan alasan (Reason*).
//
// Isinya:
//   - ticket.go: Ticket, data tiket untuk menyusun pesan.
//   - subscription.go: Subscription, tujuan dan kunci enkripsi push milik browser.
//   - event.go: Event (bentuk payload JSON NOTIFY), KindCalled, KindRecalled, KindSkipped, KindRequeued,
//     ReasonSkip, dan ReasonCounterClosed.
//
// Package ini tidak punya dependensi ke package lain di service ini, sehingga bisa dipakai semua package.
package model
