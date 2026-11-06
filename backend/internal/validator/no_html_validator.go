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
// Dipasang di cmd/server/main.go lewat RegisterValidation. Dipakai sebagai tag pada struct request.
func NoHTMLTags(
	fl validator.FieldLevel,
) bool {
	// mengambil nilai field sebagai string, lalu mengembalikan true bila tidak ada '<' atau '>'
	value := fl.Field().String()
	return !strings.ContainsAny(value, "<>")
}
