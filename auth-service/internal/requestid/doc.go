// Package requestid menyimpan dan membaca request ID di context.Context.
//
// Middleware RequestID mengisinya di awal request. ID dipakai untuk korelasi log dan atribut span di auth-service
// sendiri; auth-service tidak meneruskannya ke service lain.
//
// Isinya:
//   - requestid.go: With dan From.
package requestid
