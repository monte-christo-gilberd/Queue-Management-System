// Package config membaca dan memvalidasi konfigurasi payment-service dari environment variable.
//
// Load() adalah pintu masuk utama. Galat fatal hanya untuk hal yang membuat service tidak bisa berfungsi sama
// sekali (DATABASE_URL, JWT_PUBLIC_KEY_FILE, QMS_PUBLIC_ORIGIN). Kunci Stripe yang kosong bukan galat fatal: layanan tetap
// hidup dan checkout menjawab 503 (lihat StripeConfigured).
//
// Isinya:
//   - config.go: Config, Load, StripeConfigured, dan usable (menolak nilai contoh dari .env.example).
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
