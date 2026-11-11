// Package validator berisi aturan validasi kustom untuk binding request di Gin, di atas
// go-playground/validator. Nama package ini bentrok dengan library asli, jadi library diimpor dengan alias playground.
// Pemasangan dilakukan di cmd/auth-service/main.go lewat RegisterValidators.
//
// Isinya:
//   - no_html_validator.go: NoHTMLTags, aturan tag `no_html_tags` yang menolak field string berisi '<' atau '>',
//     dan RegisterValidators yang memasangnya ke validator bawaan gin.
package validator
