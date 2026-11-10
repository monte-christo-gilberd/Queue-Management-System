package export

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"qms/internal/model"
)

// Format: format berkas ekspor. CSV (comma-separated values) adalah teks biasa dengan kolom dipisah koma;
// XLSX adalah berkas spreadsheet Excel modern (Office Open XML).
type Format string

const (
	// CSV: format berkas teks dengan kolom dipisah koma.
	CSV Format = "csv"
	// XLSX: format berkas spreadsheet Excel.
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

// Extension mengembalikan ekstensi berkas tanpa titik: "csv" atau "xlsx" (sama dengan nilai Format).
func (f Format) Extension() string { return string(f) }

// ContentType mengembalikan MIME type (penanda jenis isi berkas) untuk header HTTP Content-Type.
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

// TimeLayout: format tanggal di CSV, dan format angka tanggal di XLSX. UTC (Coordinated Universal Time, zona waktu
// acuan dunia) dipakai agar hasil ekspor tidak bergantung pada zona waktu server.
const TimeLayout = "2006-01-02 15:04:05"

// Sink adalah penerima data ekspor: pemanggil mengirim tiket per halaman lewat WriteTickets, lalu memanggil Close
// untuk menyelesaikan berkas (wajib dipanggil; XLSX baru menulis byte di sini). Implementasinya ada di csv.go dan xlsx.go.
type Sink interface {
	// WriteTickets menulis satu halaman tiket ke berkas. Bisa dipanggil berkali-kali, satu kali per halaman.
	WriteTickets(tickets []model.HistoryTicket) error
	// Close menulis bagian akhir berkas dan menyelesaikannya. Wajib dipanggil setelah semua halaman ditulis.
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

// outcomeLabels memetakan hasil tiket di database (completed, skipped, dst.) ke label Indonesia yang ditulis di berkas.
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

// SafeCell mencegah formula injection (sisipan rumus: teks yang diketik pengguna dibaca sebagai rumus spreadsheet):
// sel teks yang berawalan = + - @ tab atau CR diberi awalan ' sehingga
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

// nonSlug adalah ekspresi reguler (regexp: pola pencocokan teks) untuk satu atau lebih karakter yang bukan huruf kecil
// ASCII atau angka. Dikompilasi sekali saat package dimuat, lalu dipakai FileName.
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName membuat nama berkas aman untuk Content-Disposition (header HTTP yang memberi tahu browser nama berkas unduhan):
// huruf kecil ASCII, angka, dan tanda hubung.
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

// formatTime memformat waktu sebagai teks dalam UTC sesuai TimeLayout.
func formatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// nullTime memformat waktu opsional; kosong bila tidak ada (sel dikosongkan, bukan "0001-01-01").
func nullTime(valid bool, t time.Time) string {
	if !valid {
		return ""
	}
	return formatTime(t)
}

// ticketLabel membentuk nomor tiket yang tampil, misalnya A-001: prefix (kosong dianggap "A"), lalu nomor
// tiga digit dengan nol di depan.
func ticketLabel(prefix string, number int) string {
	if prefix == "" {
		prefix = "A"
	}
	return fmt.Sprintf("%s-%03d", prefix, number)
}
