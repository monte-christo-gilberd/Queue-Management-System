// Package config membaca dan memvalidasi konfigurasi notify-service dari environment variable.
//
// Satu-satunya pintu masuk adalah Load(), yang dipanggil di cmd/notify-service/main.go. Load() gagal cepat
// bila ada variabel wajib yang kosong, sehingga container tidak jalan dengan konfigurasi setengah.
//
// Wajib: DATABASE_URL, VAPID_PUBLIC_KEY, VAPID_PRIVATE_KEY, VAPID_SUBJECT.
// Opsional: FCM_PROJECT_ID dan FCM_CREDENTIALS_FILE (path JSON service account Firebase). Kosong berarti push ke
// aplikasi Android (FCM) mati; main hanya mencatat peringatan dan Web Push tetap berjalan.
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
