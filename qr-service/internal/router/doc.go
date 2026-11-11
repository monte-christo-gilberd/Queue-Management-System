// Package router mendaftarkan semua rute HTTP qr-service dan menghubungkannya ke handler dan middleware.
//
// Cara pakai: buat AppRouter lewat NewAppRouter dengan handler wajib (probe dan token) beserta token internal
// dan logger, lalu pasang fitur opsional lewat With... (render). Fitur yang tidak dipasang tidak punya rute.
// Setelah itu SetupRouter menghasilkan *gin.Engine.
//
// Rute publik: /health dan /ready. Rute internal: /internal/v1/*, dibatasi BodyLimit dan X-Service-Token.
//
// Isinya:
//   - router.go: NewAppRouter, fungsi With..., dan SetupRouter yang mendaftarkan semua rute.
//   - router_interface.go: interface untuk setiap handler yang dipakai router, dan struct AppRouter.
//   - token_routes.go: rute token join (/join-token dan /join-token/verify).
//   - render_routes.go: rute render QR (/qr/render).
//
// Package ini hanya memetakan URL ke handler. Logika bisnis ada di service.
package router
