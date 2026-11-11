// Package router merangkai rute HTTP auth-service di atas Gin: handler diberikan lewat interface, lalu SetupRouter
// mendaftarkan rutenya.
//
// Pola yang dipakai:
//   - NewAppRouter menerima dependency wajib (handler akun, middleware login, daftar proxy tepercaya).
//   - Fitur opsional dipasang lewat With... (WithEmail, WithGoogle). Fitur yang tidak dipasang tidak punya rute,
//     jadi dijawab 404 oleh Gin.
//   - Engine dibuat dengan gin.New dan gin.Recovery (log akses dicatat nginx), dan hanya proxy di daftar tepercaya
//     yang boleh menentukan IP klien lewat X-Forwarded-For.
//
// Path publik (/api/v1/users/...) dan bentuk respons ({"data": ...} atau {"error": {"code", "message"}}) sama
// dengan yang dulu dilayani backend, jadi frontend tidak berubah. Endpoint ini dijangkau lewat nginx; backend tidak
// pernah memanggilnya.
//
// Isinya:
//   - router_interface.go: interface handler (AuthHandler, EmailHandler, GoogleHandler) dan struct AppRouter.
//   - router.go: NewAppRouter, WithEmail, WithGoogle, dan SetupRouter (termasuk rute /health).
//   - auth_routes.go: pendaftaran rute registrasi dan login, login Google, reset password dan verifikasi email,
//     serta kirim ulang verifikasi.
package router
