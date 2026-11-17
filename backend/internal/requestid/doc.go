// Package requestid menyimpan dan membaca request ID di context.Context.
//
// Middleware RequestID mengisinya di awal request, supaya lapisan bawah bisa membacanya lewat From.
//
// Isinya:
//   - requestid.go: With dan From.
package requestid
