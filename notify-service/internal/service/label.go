package service

import "fmt"

// Label membuat label nomor tiket untuk teks notifikasi, mis. "A-007". Ini satu-satunya tempat format nomor itu.
// Prefix kosong dianggap "A".
func Label(
	prefix string,
	number int,
) string {
	// Prefix kosong: pakai "A" sebagai awalan bawaan.
	if prefix == "" {
		prefix = "A"
	}
	// Gabungkan prefix dan nomor dengan tiga digit (dengan nol di depan), mis. A-007.
	return fmt.Sprintf("%s-%03d", prefix, number)
}
