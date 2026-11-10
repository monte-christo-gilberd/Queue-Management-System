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

// ParseFormat mengubah teks dari request menjadi Format. Mengembalikan ok true bila teksnya "csv" atau "xlsx".
func ParseFormat(s string) (Format, bool) {
	// memilih berdasarkan nilai s yang sudah diubah menjadi Format
	switch Format(s) {
	// s sama dengan "csv"
	case CSV:
		return CSV, true
	// s sama dengan "xlsx"
	case XLSX:
		return XLSX, true
	// s bukan format yang didukung
	default:
		return "", false
	}
}

// Extension mengembalikan ekstensi berkas tanpa titik: "csv" atau "xlsx" (sama dengan nilai Format).
func (f Format) Extension() string { return string(f) }

// ContentType mengembalikan MIME type (penanda jenis isi berkas) untuk header HTTP Content-Type.
func (f Format) ContentType() string {
	// format XLSX memakai MIME type spreadsheet Office Open XML
	if f == XLSX {
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	// selain XLSX (yaitu CSV) memakai text/csv dengan charset UTF-8
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

// NewSink membuat Sink (penerima data ekspor, lihat tipe Sink) untuk satu antrian, sesuai format f.
// Tidak ada byte yang ditulis ke w sebelum WriteTickets atau Close pertama.
func NewSink(f Format, w io.Writer, queue model.HistoryQueue) (Sink, error) {
	// memilih implementasi Sink sesuai format f
	switch f {
	// format CSV: buat csvSink, tidak ada error yang mungkin terjadi
	case CSV:
		return newCSVSink(w, queue), nil
	// format XLSX: buat xlsxSink, pembuatannya bisa gagal sehingga error ikut dikembalikan
	case XLSX:
		return newXLSXSink(w, queue)
	// format tidak dikenal: kembalikan error dan Sink kosong
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
	// ubah teks status menjadi tipe HistoryOutcome
	outcome := model.HistoryOutcome(status)
	// cari label Indonesia; ok false berarti status belum punya label
	if label, ok := outcomeLabels[outcome]; ok {
		return label
	}
	// tidak ada label: tampilkan nilai status apa adanya
	return outcome
}

// SafeCell mencegah formula injection (sisipan rumus: teks yang diketik pengguna dibaca sebagai rumus spreadsheet):
// sel teks yang berawalan = + - @ tab atau CR diberi awalan ' sehingga
// Excel atau LibreOffice tidak menjalankannya sebagai rumus bila berkas dibuka (atau dikonversi) lalu dihitung.
// Berlaku untuk teks dari pengguna (nama, email, catatan, nama counter).
func SafeCell(s string) string {
	// teks kosong tidak perlu diproteksi
	if s == "" {
		return s
	}
	// lihat karakter pertama saja, karena hanya karakter itu yang menentukan rumus
	switch s[0] {
	// karakter pertama berbahaya: beri awalan tanda kutip satu
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	// karakter pertama aman: kembalikan teks apa adanya
	return s
}

// nonSlug adalah ekspresi reguler (regexp: pola pencocokan teks) untuk satu atau lebih karakter yang bukan huruf kecil
// ASCII atau angka. Dikompilasi sekali saat package dimuat, lalu dipakai FileName.
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName membuat nama berkas aman untuk Content-Disposition (header HTTP yang memberi tahu browser nama berkas unduhan):
// huruf kecil ASCII, angka, dan tanda hubung.
// Contoh: FileName("Loket Utama!", 54, CSV) = "riwayat-loket-utama-54.csv".
func FileName(queueName string, queueID int64, f Format) string {
	// ubah ke huruf kecil, ganti karakter non-alfanumerik dengan "-", lalu buang "-" di awal dan akhir
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(queueName), "-"), "-")
	// batasi slug maksimal 40 karakter, lalu buang "-" yang mungkin tersisa di ujung
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	// nama antrian tidak menghasilkan karakter apa pun (misalnya hanya simbol): pakai nama cadangan
	if slug == "" {
		slug = "antrian"
	}
	// gabungkan menjadi riwayat-<slug>-<id>.<ekstensi>
	return fmt.Sprintf("riwayat-%s-%d.%s", slug, queueID, f.Extension())
}

// formatTime memformat waktu sebagai teks dalam UTC sesuai TimeLayout.
func formatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// nullTime memformat waktu opsional; kosong bila tidak ada (sel dikosongkan, bukan "0001-01-01").
func nullTime(valid bool, t time.Time) string {
	// valid false berarti waktu tidak pernah diisi (misalnya belum dipanggil): kembalikan teks kosong
	if !valid {
		return ""
	}
	// waktu ada: format sesuai TimeLayout
	return formatTime(t)
}

// ticketLabel membentuk nomor tiket yang tampil, misalnya A-001: prefix (kosong dianggap "A"), lalu nomor
// tiga digit dengan nol di depan.
func ticketLabel(prefix string, number int) string {
	// prefix kosong dipakai huruf "A" sebagai cadangan
	if prefix == "" {
		prefix = "A"
	}
	// %03d: angka dengan minimal tiga digit, diisi nol di depan
	return fmt.Sprintf("%s-%03d", prefix, number)
}
