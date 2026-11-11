package validator

import (
	"strings"

	"github.com/gin-gonic/gin/binding"
	playground "github.com/go-playground/validator/v10"
)

// NoHTMLTags adalah validator kustom dengan tag `no_html_tags`: menolak field string yang mengandung '<' atau '>'.
// Ini pemeriksaan sederhana, bukan parser HTML, dan tidak membersihkan (sanitize) isinya.
func NoHTMLTags(
	fl playground.FieldLevel,
) bool {
	// ContainsAny true bila ada '<' atau '>'; dibalik agar validasi lolos hanya saat keduanya tidak ada
	return !strings.ContainsAny(fl.Field().String(), "<>")
}

// RegisterValidators memasang aturan `no_html_tags` ke validator bawaan gin.
func RegisterValidators() {
	// ambil engine validator gin; bila bukan go-playground, tidak ada yang dipasang
	if v, ok := binding.Validator.Engine().(*playground.Validate); ok {
		// pasang aturan no_html_tags (galat pendaftaran diabaikan, sama seperti sebelumnya)
		_ = v.RegisterValidation("no_html_tags", NoHTMLTags)
	}
}
