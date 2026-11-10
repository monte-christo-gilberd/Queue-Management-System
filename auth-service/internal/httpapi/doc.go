// Package httpapi: rute HTTP auth-service (Gin).
//
// Isinya:
//   - httpapi.go: DTO request, NewRouter, middleware requireAuth, dan handler register, login, lupa/reset password,
//     verifikasi email, dan kirim ulang verifikasi.
//
// Path publik (/api/v1/users/...) dan bentuk respons ({"data": ...} atau {"error": {"code", "message"}}) sama
// dengan yang dulu dilayani backend, jadi frontend tidak berubah. Endpoint ini dijangkau lewat nginx; backend tidak
// pernah memanggilnya.
package httpapi
