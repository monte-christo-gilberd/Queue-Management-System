// Package mailtemplate mengisi templat email (subjek, teks, dan HTML) dengan data dari pemanggil. Berkas templat
// dibenamkan ke binary lewat go:embed dan ada di folder templates/.
//
// Aturan yang dipakai:
//   - Setiap email punya tiga berkas: <nama>.subject.tmpl, <nama>.txt.tmpl, dan <nama>.html.tmpl. HTML dibungkus
//     layout.html.tmpl dan memakai html/template, jadi nilai dari pengguna otomatis di-escape.
//   - Kunci data yang hilang adalah galat (apperror.ErrBadData), bukan "<no value>" yang diam-diam terkirim.
//   - Nama templat dibatasi huruf kecil, angka, dan tanda hubung, sehingga tidak bisa keluar dari folder templates/.
//
// Isinya:
//   - renderer.go: Renderer (New, Has, Render) beserta direktif go:embed.
//   - templates/: layout.html.tmpl dan templat email (cancelled, completed, joined, removed, email-verify,
//     password-reset, queue-closed, email-verified, test).
//
// Package ini tidak mengirim email. Pengiriman ada di package mailer, dan pemanggilnya adalah service.
package mailtemplate
