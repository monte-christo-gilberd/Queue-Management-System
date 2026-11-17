// Package router mendaftarkan semua rute HTTP di bawah /api/v1 dan menghubungkannya ke handler dan middleware.
//
// Cara pakai: buat AppRouter lewat NewAppRouter dengan handler wajib (tiket, user, queue, auth, SSE), lalu pasang
// fitur opsional lewat With... (counter, catatan, riwayat, reset password, verifikasi email, layar publik, push, push FCM).
// Fitur yang tidak dipasang tidak punya rute sama sekali. Setelah itu SetupRouter menghasilkan *gin.Engine.
//
// Istilah yang dipakai:
//   - Handler: objek yang menerima request dari Gin (*gin.Context) dan mengembalikan respons. Di package ini,
//     handler dinyatakan sebagai interface agar router tidak bergantung pada implementasinya.
//   - Middleware: fungsi yang dijalankan sebelum handler, misalnya memeriksa login (authMiddleware) atau CORS.
//
// Isinya:
//   - router.go: AppRouter, NewAppRouter, fungsi With..., dan SetupRouter yang mendaftarkan semua rute.
//   - router_interface.go: interface untuk setiap handler yang dipakai router.
//   - *_routes.go: pendaftaran rute per kelompok fitur (user, queue, SSE, catatan, counter, riwayat).
//
// Package ini hanya memetakan URL ke handler. Logika bisnis ada di service, dan pengecekan kepemilikan ada di service.
package router
