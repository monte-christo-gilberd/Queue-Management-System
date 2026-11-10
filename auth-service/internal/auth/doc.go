// Package auth berisi logika akun auth-service.
//
// Isinya:
//   - service.go: Service (registrasi, login, Authenticate, reset password, verifikasi email) beserta antarmuka
//     UserStore, TokenStore, dan Mailer yang dipenuhi store dan mailclient (jadi bisa dites dengan fake tanpa database).
//   - tokens.go: TokenIssuer, penerbit dan pemeriksa JWT RS256. Kunci privat hanya ada di layanan ini.
//
// Package ini tidak tahu apa pun tentang HTTP. Pemetaan galat ke status HTTP ada di internal/httpapi.
package auth
