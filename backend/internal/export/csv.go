package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"qms/internal/model"
)

// utf8BOM (byte order mark) adalah tiga byte di awal berkas yang memberi tahu Excel bahwa isinya UTF-8,
// sehingga nama dan catatan berbahasa Indonesia tidak rusak.
const utf8BOM = "\xEF\xBB\xBF"

// csvSink adalah Sink untuk CSV: menerima tiket per halaman lalu menuliskannya sebagai baris CSV.
// Baris ditulis ke csv.Writer (buffer kecil dari library standar) dan di-Flush
// setiap halaman, sehingga data terus dikirim ke writer tanpa menumpuk di memori.
type csvSink struct {
	// w adalah tujuan tulis akhir (misalnya response HTTP).
	w io.Writer
	// cw adalah penulis CSV dari library standar encoding/csv.
	cw *csv.Writer
	// prefix adalah awalan nomor tiket dari antrian (misalnya "A").
	prefix string
	// started menandai bahwa BOM dan judul kolom sudah ditulis, supaya tidak ditulis dua kali.
	started bool
}

// newCSVSink membuat Sink CSV untuk antrian queue yang menulis ke w.
func newCSVSink(w io.Writer, queue model.HistoryQueue) *csvSink {
	// buat csv.Writer yang menulis ke w
	cw := csv.NewWriter(w)
	// pakai CRLF (carriage return + line feed) sebagai akhir baris, karena itu format baris Windows dan Excel di Windows nyaman membacanya
	cw.UseCRLF = true
	// kembalikan csvSink dengan prefix nomor tiket dari antrian
	return &csvSink{w: w, cw: cw, prefix: queue.TicketPrefix}
}

// begin menulis BOM dan judul kolom sekali, sebelum baris pertama (juga untuk antrian kosong).
func (s *csvSink) begin() error {
	// sudah pernah ditulis: tidak perlu menulis lagi
	if s.started {
		return nil
	}
	// tandai sudah mulai agar pemanggilan berikutnya dilewati
	s.started = true
	// tulis BOM langsung ke writer; jika gagal, kembalikan error
	if _, err := io.WriteString(s.w, utf8BOM); err != nil {
		return err
	}
	// tulis judul kolom sebagai baris pertama CSV
	return s.cw.Write(Headers)
}

// WriteTickets menulis satu baris CSV per tiket dalam halaman ini. Memanggil begin dulu agar judul kolom tertulis.
func (s *csvSink) WriteTickets(tickets []model.HistoryTicket) error {
	// pastikan BOM dan judul sudah tertulis sebelum baris data; jika gagal, hentikan
	if err := s.begin(); err != nil {
		return err
	}
	// tulis setiap tiket di halaman ini sebagai satu baris
	for _, t := range tickets {
		// ubah tiket menjadi baris CSV; jika penulisan baris gagal, hentikan
		if err := s.cw.Write(csvRow(s.prefix, t)); err != nil {
			return err
		}
	}
	// kosongkan buffer csv.Writer ke writer agar data halaman ini langsung terkirim
	s.cw.Flush()
	// kembalikan error yang terjadi saat menulis (nil bila semuanya berhasil)
	return s.cw.Error()
}

// Close memastikan BOM dan judul sudah tertulis (untuk antrian tanpa tiket), lalu mengosongkan buffer ke writer.
func (s *csvSink) Close() error {
	// antrian tanpa tiket belum pernah memanggil begin, jadi panggil di sini; jika gagal, hentikan
	if err := s.begin(); err != nil {
		return err
	}
	// kosongkan sisa buffer ke writer
	s.cw.Flush()
	// kembalikan error yang terjadi saat menulis (nil bila semuanya berhasil)
	return s.cw.Error()
}

// csvRow mengubah satu tiket menjadi satu baris CSV sesuai urutan Headers. Teks dari pengguna lewat SafeCell,
// dan waktu opsional yang kosong dikosongkan lewat nullTime.
func csvRow(prefix string, t model.HistoryTicket) []string {
	// susun kolom sesuai urutan Headers
	return []string{
		// nomor tiket, misalnya A-001
		ticketLabel(prefix, t.TicketNumber),
		// nama dan email pelanggan, diamankan dari formula injection
		SafeCell(t.CustomerName),
		SafeCell(t.CustomerEmail),
		// hasil tiket dalam bahasa Indonesia
		OutcomeLabel(t.Status),
		// waktu bergabung selalu ada
		formatTime(t.JoinedAt),
		// waktu dipanggil, selesai, dan dibatalkan: kosong bila Valid false
		nullTime(t.CalledAt.Valid, t.CalledAt.Time),
		nullTime(t.CompletedAt.Valid, t.CompletedAt.Time),
		nullTime(t.CancelledAt.Valid, t.CancelledAt.Time),
		// jumlah lewati diubah menjadi teks
		strconv.Itoa(t.SkipCount),
		// nama counter dan catatan, diamankan dari formula injection
		SafeCell(t.CounterName.String),
		SafeCell(t.Note.String),
	}
}
