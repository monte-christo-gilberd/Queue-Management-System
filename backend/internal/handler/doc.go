// Package handler berisi handler HTTP (Gin) yang menerima request, memeriksa input, memanggil service,
// lalu menerjemahkan hasilnya menjadi respons JSON.
//
// Pola yang dipakai di semua handler:
//   - Input diambil dari URL (c.Param), query (c.Query), atau body JSON (ShouldBindJSON). Input yang salah
//     langsung dibalas 400 BAD_REQUEST.
//   - Error dari service diterjemahkan lewat sentinel error di apperror (errors.Is) menjadi kode HTTP dan
//     kode error yang tetap, misalnya 404 NOT_FOUND atau 409 QUEUE_CLOSED.
//   - Error yang tidak dikenal dibalas 500 INTERNAL_ERROR tanpa detail, supaya pesan teknis tidak bocor ke klien.
//   - Respons sukses dibungkus di field "data" lewat respondData, dan respons error di field "error" lewat respondError.
//
// Isinya:
//   - response.go: respondData dan respondError, dua pembantu untuk bentuk respons JSON.
//   - handler_interface.go: interface handler yang dipakai router.
//   - user_handler.go, ticket_handler.go, queue_handler.go, counter_handler.go, display_handler.go,
//     history_handler.go, note_handler.go, sse_handler.go, push_handler.go, password_handler.go,
//     email_verification_handler.go: handler per fitur.
//
// Handler hanya mengatur HTTP. Aturan bisnis, termasuk pengecekan kepemilikan antrian, ada di package service.
package handler
