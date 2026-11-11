package mailtemplate

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"strings"
	texttemplate "text/template"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/model"
)

// templateFS: berkas templat email, dibenamkan ke dalam binary saat build.
//
//go:embed templates/*.tmpl
var templateFS embed.FS

// nameRe: aturan nama templat yang sah: huruf kecil, angka, dan tanda hubung, 1 sampai 41 karakter.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

// Renderer mengisi templat email dari berkas yang dibenamkan. Dibuat lewat New.
type Renderer struct{}

// New membuat Renderer.
func New() *Renderer {
	// Renderer tidak punya state: semua templat dibaca dari templateFS.
	return &Renderer{}
}

// Has melaporkan apakah templat dengan nama tersebut lengkap (subjek, teks, dan HTML) dan namanya sah.
func (r *Renderer) Has(
	name string,
) bool {
	// Nama tidak sesuai aturan, atau nama "layout" (hanya dipakai sebagai pembungkus): bukan templat email.
	if !nameRe.MatchString(name) || name == "layout" {
		// Nama tidak sah ditolak sebelum dipakai membentuk path berkas, mencegah path traversal.
		return false
	}
	// Ketiga berkas harus ada di dalam binary.
	for _, ext := range []string{".subject.tmpl", ".txt.tmpl", ".html.tmpl"} {
		// Salah satu berkas hilang: templat dianggap tidak lengkap.
		if _, err := templateFS.ReadFile("templates/" + name + ext); err != nil {
			// Berkas hilang: templat tidak lengkap.
			return false
		}
	}
	// Semua berkas ada.
	return true
}

// Render mengisi templat <name>. Templat HTML memakai html/template, jadi nilai dari pengguna (nama antrian,
// nama customer) otomatis di-escape; teks dan subjek tidak di-escape. Kunci data yang hilang adalah galat
// (apperror.ErrBadData), bukan "<no value>" yang diam-diam terkirim ke pelanggan.
func (r *Renderer) Render(
	name string,
	data map[string]any,
) (model.Rendered, error) {
	// Templat tidak ada atau tidak lengkap: kembalikan galat sebelum memproses apa pun.
	if !r.Has(name) {
		// Dipetakan handler menjadi 400 UNKNOWN_TEMPLATE.
		return model.Rendered{}, apperror.ErrUnknownTemplate
	}
	// Opsi yang membuat kunci data yang hilang menjadi galat, bukan teks kosong.
	opt := "missingkey=error"
	// Muat templat subjek (teks biasa).
	subj, err := texttemplate.New("subject").Option(opt).ParseFS(templateFS, "templates/"+name+".subject.tmpl")
	if err != nil {
		// Galat urai berasal dari templat di dalam binary, jadi kesalahan server (bukan data).
		return model.Rendered{}, fmt.Errorf("subject template %q: %w", name, err)
	}
	// Muat templat teks biasa.
	text, err := texttemplate.New("text").Option(opt).ParseFS(templateFS, "templates/"+name+".txt.tmpl")
	if err != nil {
		// Galat urai berasal dari templat di dalam binary, jadi kesalahan server (bukan data).
		return model.Rendered{}, fmt.Errorf("text template %q: %w", name, err)
	}
	// Muat templat HTML bersama layout. html/template meng-escape nilai secara otomatis.
	html, err := htmltemplate.New("html").Option(opt).ParseFS(templateFS, "templates/layout.html.tmpl", "templates/"+name+".html.tmpl")
	if err != nil {
		// Galat urai berasal dari templat di dalam binary, jadi kesalahan server (bukan data).
		return model.Rendered{}, fmt.Errorf("html template %q: %w", name, err)
	}
	// Buffer untuk hasil render subjek, teks, dan HTML.
	var s, t, h bytes.Buffer
	// Isi subjek dengan data. Galat (mis. kunci hilang) dianggap data tidak lengkap.
	if err := subj.ExecuteTemplate(&s, name+".subject.tmpl", data); err != nil {
		// Dibungkus ErrBadData agar handler membalas 400, bukan 500.
		return model.Rendered{}, fmt.Errorf("%w: %v", apperror.ErrBadData, err)
	}
	// Isi versi teks dengan data.
	if err := text.ExecuteTemplate(&t, name+".txt.tmpl", data); err != nil {
		return model.Rendered{}, fmt.Errorf("%w: %v", apperror.ErrBadData, err)
	}
	// Isi versi HTML dengan layout sebagai pembungkus.
	if err := html.ExecuteTemplate(&h, "layout", data); err != nil {
		return model.Rendered{}, fmt.Errorf("%w: %v", apperror.ErrBadData, err)
	}
	// Subjek satu baris: CR/LF dari data dibuang supaya tidak bisa menyisipkan header.
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(s.String()))
	// Kembalikan tiga bagian email yang sudah terisi.
	return model.Rendered{Subject: subject, Text: t.String(), HTML: h.String()}, nil
}
