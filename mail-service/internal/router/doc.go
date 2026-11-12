// Package router mendaftarkan semua rute HTTP mail-service di atas gin dan menghubungkannya ke
// handler dan middleware.
//
// Cara pakai: buat AppRouter lewat NewAppRouter dengan handler wajib (kirim email) dan middleware token layanan,
// lalu pasang fitur opsional lewat With... (WithSystem untuk /health dan 404). Setelah itu SetupRouter
// menghasilkan *gin.Engine (sebuah http.Handler). Fitur yang tidak dipasang tidak punya rute sama sekali.
//
// Rute yang terdaftar:
//   - GET /health: tanpa token.
//   - POST /internal/v1/send: satu-satunya rute yang dilindungi X-Service-Token.
//   - rute lain: 404 NOT_FOUND.
//
// Isinya:
//   - router.go: AppRouter, NewAppRouter, WithSystem, dan SetupRouter yang mendaftarkan semua rute.
//   - router_interface.go: interface handler yang dipakai router.
//   - mail_routes.go: pendaftaran rute kirim email.
//
// Package ini hanya memetakan URL ke handler. Logika bisnis ada di service.
package router
