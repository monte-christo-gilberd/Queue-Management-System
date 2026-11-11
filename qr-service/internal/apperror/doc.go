// Package apperror berisi sentinel error yang dipakai bersama oleh service, handler, dan middleware.
//
// Service mengembalikan error dari package ini (dibungkus dengan %w bila perlu). Handler menerjemahkannya
// lewat errors.Is menjadi kode HTTP dan kode error yang tetap. Dengan begitu service tidak perlu mengenal HTTP.
//
// Isinya:
//   - errors.go: semua sentinel error, dikelompokkan menjadi error token dan error input.
package apperror
