package mailer

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"strings"
	texttemplate "text/template"
)

// templateFS: berkas templat email, dibenamkan ke dalam binary saat build.
//
//go:embed templates/*.tmpl
var templateFS embed.FS

// Rendered: isi satu email dari tiga berkas templat: <nama>.subject.tmpl, <nama>.txt.tmpl, <nama>.html.tmpl.
type Rendered struct {
	// Subject: subjek satu baris, sudah dibersihkan dari CR dan LF.
	Subject string
	// Text: isi versi teks biasa.
	Text string
	// HTML: isi versi HTML, dibungkus layout.html.tmpl.
	HTML string
}

// ErrUnknownTemplate: nama templat tidak ada atau tidak sah. ErrBadData: data tidak melengkapi templat.
var (
	// ErrUnknownTemplate: nama templat tidak ada, tidak sah, atau berkasnya tidak lengkap.
	ErrUnknownTemplate = errors.New("unknown email template")
	// ErrBadData: data tidak melengkapi kunci yang dibutuhkan templat.
	ErrBadData = errors.New("template data is incomplete")
)

// nameRe: aturan nama templat yang sah: huruf kecil, angka, dan tanda hubung, 1 sampai 41 karakter.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

// Has melaporkan apakah templat dengan nama tersebut lengkap (subjek, teks, dan HTML) dan namanya sah.
func Has(name string) bool {
	// Nama tidak sesuai aturan, atau nama "layout" (hanya dipakai sebagai pembungkus): bukan templat email.
	if !nameRe.MatchString(name) || name == "layout" {
		return false
	}
	// Ketiga berkas harus ada di dalam binary.
	for _, ext := range []string{".subject.tmpl", ".txt.tmpl", ".html.tmpl"} {
		// Salah satu berkas hilang: templat dianggap tidak lengkap.
		if _, err := templateFS.ReadFile("templates/" + name + ext); err != nil {
			return false
		}
	}
	return true
}

// Render mengisi templat <name>. Templat HTML memakai html/template, jadi nilai dari pengguna (nama antrian,
// nama customer) otomatis di-escape; teks dan subjek tidak di-escape. Kunci data yang hilang adalah galat
// (ErrBadData), bukan "<no value>" yang diam-diam terkirim ke pelanggan.
func Render(name string, data map[string]any) (Rendered, error) {
	// Templat tidak ada atau tidak lengkap: kembalikan galat sebelum memproses apa pun.
	if !Has(name) {
		return Rendered{}, ErrUnknownTemplate
	}
	// Opsi yang membuat kunci data yang hilang menjadi galat, bukan teks kosong.
	opt := "missingkey=error"
	// Muat templat subjek (teks biasa).
	subj, err := texttemplate.New("subject").Option(opt).ParseFS(templateFS, "templates/"+name+".subject.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("subject template %q: %w", name, err)
	}
	// Muat templat teks biasa.
	text, err := texttemplate.New("text").Option(opt).ParseFS(templateFS, "templates/"+name+".txt.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("text template %q: %w", name, err)
	}
	// Muat templat HTML bersama layout. html/template meng-escape nilai secara otomatis.
	html, err := htmltemplate.New("html").Option(opt).ParseFS(templateFS, "templates/layout.html.tmpl", "templates/"+name+".html.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("html template %q: %w", name, err)
	}
	// Buffer untuk hasil render subjek, teks, dan HTML.
	var s, t, h bytes.Buffer
	// Isi subjek dengan data. Galat (mis. kunci hilang) dianggap data tidak lengkap.
	if err := subj.ExecuteTemplate(&s, name+".subject.tmpl", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	// Isi versi teks dengan data.
	if err := text.ExecuteTemplate(&t, name+".txt.tmpl", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	// Isi versi HTML dengan layout sebagai pembungkus.
	if err := html.ExecuteTemplate(&h, "layout", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	// Subjek satu baris: CR/LF dari data dibuang supaya tidak bisa menyisipkan header.
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(s.String()))
	return Rendered{Subject: subject, Text: t.String(), HTML: h.String()}, nil
}
