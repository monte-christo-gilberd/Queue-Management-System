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

//go:embed templates/*.tmpl
var templateFS embed.FS

// Rendered: isi satu email dari tiga berkas templat: <nama>.subject.tmpl, <nama>.txt.tmpl, <nama>.html.tmpl.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

// ErrUnknownTemplate: nama templat tidak ada atau tidak sah. ErrBadData: data tidak melengkapi templat.
var (
	ErrUnknownTemplate = errors.New("unknown email template")
	ErrBadData         = errors.New("template data is incomplete")
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

// Has: true bila templat <name> lengkap (subjek, teks, HTML) ada.
func Has(name string) bool {
	if !nameRe.MatchString(name) || name == "layout" {
		return false
	}
	for _, ext := range []string{".subject.tmpl", ".txt.tmpl", ".html.tmpl"} {
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
	if !Has(name) {
		return Rendered{}, ErrUnknownTemplate
	}
	opt := "missingkey=error"
	subj, err := texttemplate.New("subject").Option(opt).ParseFS(templateFS, "templates/"+name+".subject.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("subject template %q: %w", name, err)
	}
	text, err := texttemplate.New("text").Option(opt).ParseFS(templateFS, "templates/"+name+".txt.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("text template %q: %w", name, err)
	}
	html, err := htmltemplate.New("html").Option(opt).ParseFS(templateFS, "templates/layout.html.tmpl", "templates/"+name+".html.tmpl")
	if err != nil {
		return Rendered{}, fmt.Errorf("html template %q: %w", name, err)
	}
	var s, t, h bytes.Buffer
	if err := subj.ExecuteTemplate(&s, name+".subject.tmpl", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	if err := text.ExecuteTemplate(&t, name+".txt.tmpl", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	if err := html.ExecuteTemplate(&h, "layout", data); err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadData, err)
	}
	// Subjek satu baris: CR/LF dari data dibuang supaya tidak bisa menyisipkan header.
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(s.String()))
	return Rendered{Subject: subject, Text: t.String(), HTML: h.String()}, nil
}
