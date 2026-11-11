// Package middleware berisi handler middleware Gin yang dipasang di router sebelum handler endpoint.
//
// Middleware dibuat di cmd/auth-service/main.go (Auth) lalu diberikan ke router.NewAppRouter.
//
// Isinya:
//   - auth.go: Auth, yang memeriksa JWT RS256 lewat interface Authenticator (token tidak sah atau sesi dicabut: 401),
//     lalu menyimpan id user di context dengan kunci "user_id".
//   - abort_response.go: abortError, pembantu untuk menghentikan request dengan body JSON error yang seragam.
package middleware
