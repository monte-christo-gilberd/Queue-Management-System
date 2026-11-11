// Package handler berisi handler HTTP (Gin) auth-service yang menerima request, memeriksa input, memanggil service,
// lalu menerjemahkan hasilnya menjadi respons JSON.
//
// Pola yang dipakai di semua handler:
//   - Input diambil dari body JSON (ShouldBindJSON). Input yang salah langsung dibalas 400 BAD_REQUEST.
//   - Error dari service diterjemahkan lewat sentinel error di apperror (errors.Is) di fungsi respondXError per
//     operasi, menjadi kode HTTP dan kode error yang tetap, misalnya 400 WEAK_PASSWORD atau 409 CONFLICT.
//   - Error yang tidak dikenal dibalas 500 INTERNAL_ERROR tanpa detail, supaya pesan teknis tidak bocor ke klien.
//   - Respons sukses dibungkus di field "data" lewat respondData, dan respons error di field "error" lewat respondError.
//
// Isinya:
//   - response.go: respondData dan respondError, dua pembantu untuk bentuk respons JSON.
//   - handler_interface.go: AuthHandlerInterface, kontrak handler yang dipakai router.
//   - auth_handler.go: AuthHandler (register, login, lupa dan reset password, verifikasi email, kirim ulang
//     verifikasi, login Google, tautkan Google) beserta interface AuthService yang dipakainya.
//
// Handler hanya mengatur HTTP. Aturan bisnis ada di package service. Pengiriman email reset password berjalan di
// goroutine yang dibatasi semafor, supaya mail-service yang macet tidak menumpuk goroutine.
package handler
