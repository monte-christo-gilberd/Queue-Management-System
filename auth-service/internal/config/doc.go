// Package config membaca environment variable auth-service, menerapkan nilai bawaan (PORT 8094, SHUTDOWN_TIMEOUT,
// AUTH_JWT_KID, EMAIL_VERIFICATION_REQUIRED), memuat kunci privat JWT dari berkas PEM, lalu menolak konfigurasi
// yang kurang atau tidak aman saat APP_ENV=production. Dipakai oleh cmd/auth-service/main.go.
package config
