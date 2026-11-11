// Package handler berisi handler HTTP (Gin) yang menerima request, membaca body JSON, memanggil service,
// lalu menerjemahkan hasilnya menjadi respons.
//
// Pola yang dipakai di semua handler:
//   - Input diambil dari body JSON (ShouldBindJSON). Body yang tidak bisa dibaca langsung dibalas 400 BAD_REQUEST.
//   - Error dari service diterjemahkan lewat sentinel error di apperror (errors.Is) menjadi kode HTTP dan
//     kode error yang tetap, misalnya 410 QR_EXPIRED atau 401 QR_INVALID.
//   - Respons sukses adalah JSON polos tanpa pembungkus "data" (respondJSON), atau PNG (respondPNG).
//     Respons error dibungkus di field "error" lewat respondError.
//
// Isinya:
//   - response.go: respondJSON, respondPNG, dan respondError, tiga pembantu untuk bentuk respons.
//   - handler_interface.go: interface handler yang dipakai router.
//   - health_handler.go: HealthHandler (Health dan Ready) untuk probe container.
//   - token_handler.go: TokenHandler (IssueToken dan VerifyToken) beserta interface TokenService.
//   - render_handler.go: RenderHandler (Render) beserta interface RenderService.
//
// Handler hanya mengatur HTTP. Aturan bisnis token dan render ada di package service.
package handler
