package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

// NoHTMLTags adalah validator kustom untuk go-playground/validator dengan tag `no_html_tags`.
// Fungsi ini menolak field string yang mengandung karakter '<' atau '>', sehingga input seperti
// "<script>" tidak lolos. Ini pemeriksaan sederhana: hanya memeriksa dua karakter itu, bukan
// mem-parse HTML, dan tidak membersihkan (sanitize) isinya.
//
// Dipasang di internal/app lewat RegisterValidation. Dipakai sebagai tag pada struct request.
func NoHTMLTags(
	fl validator.FieldLevel,
) bool {
	// ambil nilai field yang sedang divalidasi sebagai string
	value := fl.Field().String()
	// ContainsAny true bila ada '<' atau '>' di value; dibalik agar validasi lolos hanya saat keduanya tidak ada
	return !strings.ContainsAny(value, "<>")
}
