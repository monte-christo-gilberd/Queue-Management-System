// Package service berisi logika bisnis auth-service: registrasi, login, pemeriksaan token, reset password,
// verifikasi email, dan login Google. Handler HTTP memanggil service, lalu service memanggil repository.
//
// Aturan yang dipakai di semua service:
//   - Service tidak mengenal HTTP. Ia menerima context dan data biasa, lalu mengembalikan hasil atau apperror.
//     Handler yang menerjemahkan error itu menjadi kode HTTP.
//   - Service hanya bergantung pada interface (UserRepository, TokenRepository, Mailer, GoogleVerifier) yang
//     dideklarasikan di service_interface.go, jadi bisa dites dengan fake tanpa database, SMTP, atau jaringan.
//   - Token sekali pakai (reset password dan verifikasi email) hanya dikirim lewat email dalam bentuk mentah;
//     di database hanya hash SHA-256-nya yang disimpan.
//   - Akun yang tidak ada atau yang sudah mencapai batas per jam tidak dibedakan dari yang berhasil (tidak ada
//     enumerasi akun).
//
// Pola konstruktor: NewService menerima dependency wajib. Login Google dipasang lewat WithGoogle; tanpa itu login
// Google mati.
//
// Isinya:
//   - service_interface.go: UserRepository, TokenRepository, Mailer, dan GoogleVerifier yang dipakai service.
//   - auth_service.go: Service, Options, NewService, ValidatePassword, Register, Login, dan Authenticate.
//   - password_service.go: RequestReset dan ResetPassword, serta pembuat dan hash token sekali pakai.
//   - verification_service.go: SendVerification, VerifyEmail, dan pengiriman verifikasi di latar belakang.
//   - google_service.go: WithGoogle, LoginWithGoogle, LinkGoogle, LinkRequiredError, dan pembuatan username.
//   - link_attempts.go: penghitung salah password per link token Google (di memori, satu per Service).
//   - token_issuer.go: TokenIssuer, penerbit dan pemeriksa JWT RS256 (token login dan link token Google).
package service
