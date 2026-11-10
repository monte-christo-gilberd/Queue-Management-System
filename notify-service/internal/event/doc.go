// Package event mengurai payload NOTIFY dari trigger tickets_notify di database dan mendengarkannya
// lewat perintah LISTEN queue_events.
//
// Isinya:
//   - event.go: Event (bentuk payload JSON), konstanta jenis (KindCalled, KindRecalled, KindSkipped,
//     KindRequeued) dan alasan (ReasonSkip, ReasonCounterClosed), Parse (mengurai dan memeriksa payload),
//     serta Listen (koneksi LISTEN yang berjalan sampai koneksi putus atau context selesai).
//
// Package ini tidak tahu siapa yang harus diberi tahu. Itu urusan package notify.
package event
