// Package validator berisi aturan validasi kustom untuk binding request di Gin, di atas
// go-playground/validator. Package ini hanya mendefinisikan aturan dan tidak memasangnya sendiri:
// pemasangan dilakukan di cmd/server/main.go. Nama package ini bentrok dengan library asli, jadi
// diimpor dengan alias noHTMLValidator.
package validator