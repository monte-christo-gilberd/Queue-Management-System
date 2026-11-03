// Package export menulis riwayat antrian sebagai CSV atau XLSX (4.15).
// Keduanya ditulis bertahap (streaming) per halaman tiket, supaya antrian besar tidak dimuat sekaligus ke memori.
package export

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"qms/internal/model"
)

// Format: format berkas ekspor.
type Format string

const (
	CSV  Format = "csv"
	XLSX Format = "xlsx"
)

// ParseFormat menerima "csv" atau "xlsx" (persis, huruf kecil). Selain itu: ok false.
func ParseFormat(s string) (Format, bool) {
	switch Format(s) {
	case CSV:
		return CSV, true
	case XLSX:
		return XLSX, true
	default:
		return "", false
	}
}

func (f Format) Extension() string { return string(f) }

func (f Format) ContentType() string {
	if f == XLSX {
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	return "text/csv; charset=utf-8"
}

// Headers: judul kolom, dalam urutan yang sama dengan sel yang ditulis sink.
var Headers = []string{
	"Nomor", "Nama", "Email", "Hasil",
	"Bergabung (UTC)", "Dipanggil (UTC)", "Selesai (UTC)", "Dibatalkan (UTC)",
	"Jumlah Dilewati", "Counter", "Catatan",
}

// TimeLayout: format tanggal di CSV, dan format angka tanggal di XLSX. UTC agar tidak bergantung zona server.
const TimeLayout = "2006-01-02 15:04:05"

// Sink menerima tiket per halaman. Close menyelesaikan berkas (wajib dipanggil; XLSX baru menulis byte di sini).
type Sink interface {
	WriteTickets(tickets []model.HistoryTicket) error
	Close() error
}

// NewSink membuat sink untuk satu antrian. Tidak ada byte yang ditulis sebelum WriteTickets atau Close pertama.
func NewSink(f Format, w io.Writer, queue model.HistoryQueue) (Sink, error) {
	switch f {
	case CSV:
		return newCSVSink(w, queue), nil
	case XLSX:
		return newXLSXSink(w, queue)
	default:
		return nil, fmt.Errorf("export: format tidak dikenal %q", f)
	}
}

var outcomeLabels = map[string]string{
	"completed": "Selesai",
	"skipped":   "Dilewati",
	"cancelled": "Dibatalkan",
	"deleted":   "Dihapus",
	"unserved":  "Belum dilayani",
}

// OutcomeLabel: label Indonesia untuk hasil akhir (sama dengan yang ditampilkan di layar riwayat).
func OutcomeLabel(status string) string {
	outcome := model.HistoryOutcome(status)
	if label, ok := outcomeLabels[outcome]; ok {
		return label
	}
	return outcome
}

// SafeCell mencegah formula injection: sel teks yang berawalan = + - @ tab atau CR diberi awalan ' sehingga
// Excel atau LibreOffice tidak menjalankannya sebagai rumus bila berkas dibuka (atau dikonversi) lalu dihitung.
// Berlaku untuk teks dari pengguna (nama, email, catatan, nama counter).
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName membuat nama berkas aman untuk Content-Disposition: huruf kecil ASCII, angka, dan tanda hubung.
// Contoh: FileName("Loket Utama!", 54, CSV) = "riwayat-loket-utama-54.csv".
func FileName(queueName string, queueID int64, f Format) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(queueName), "-"), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		slug = "antrian"
	}
	return fmt.Sprintf("riwayat-%s-%d.%s", slug, queueID, f.Extension())
}

func formatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// nullTime memformat waktu opsional; kosong bila tidak ada (sel dikosongkan, bukan "0001-01-01").
func nullTime(valid bool, t time.Time) string {
	if !valid {
		return ""
	}
	return formatTime(t)
}

func ticketLabel(prefix string, number int) string {
	if prefix == "" {
		prefix = "A"
	}
	return fmt.Sprintf("%s-%03d", prefix, number)
}
