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

// csvSink adalah Sink untuk CSV. Baris ditulis ke csv.Writer (buffer kecil dari library standar) dan di-Flush
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

// newCSVSink membuat csvSink untuk antrian queue yang menulis ke w.
func newCSVSink(w io.Writer, queue model.HistoryQueue) *csvSink {
	cw := csv.NewWriter(w)
	cw.UseCRLF = true // CRLF (carriage return + line feed) adalah akhir baris Windows; dipakai agar Excel di Windows nyaman
	return &csvSink{w: w, cw: cw, prefix: queue.TicketPrefix}
}

// begin menulis BOM dan judul kolom sekali, sebelum baris pertama (juga untuk antrian kosong).
func (s *csvSink) begin() error {
	if s.started {
		return nil
	}
	s.started = true
	if _, err := io.WriteString(s.w, utf8BOM); err != nil {
		return err
	}
	return s.cw.Write(Headers)
}

// WriteTickets menulis satu baris CSV per tiket dalam halaman ini. Memanggil begin dulu agar judul kolom tertulis.
func (s *csvSink) WriteTickets(tickets []model.HistoryTicket) error {
	if err := s.begin(); err != nil {
		return err
	}
	for _, t := range tickets {
		if err := s.cw.Write(csvRow(s.prefix, t)); err != nil {
			return err
		}
	}
	s.cw.Flush()
	return s.cw.Error()
}

// Close memastikan BOM dan judul sudah tertulis (untuk antrian tanpa tiket), lalu mengosongkan buffer ke writer.
func (s *csvSink) Close() error {
	if err := s.begin(); err != nil {
		return err
	}
	s.cw.Flush()
	return s.cw.Error()
}

// csvRow mengubah satu tiket menjadi satu baris CSV sesuai urutan Headers. Teks dari pengguna lewat SafeCell,
// dan waktu opsional yang kosong dikosongkan lewat nullTime.
func csvRow(prefix string, t model.HistoryTicket) []string {
	return []string{
		ticketLabel(prefix, t.TicketNumber),
		SafeCell(t.CustomerName),
		SafeCell(t.CustomerEmail),
		OutcomeLabel(t.Status),
		formatTime(t.JoinedAt),
		nullTime(t.CalledAt.Valid, t.CalledAt.Time),
		nullTime(t.CompletedAt.Valid, t.CompletedAt.Time),
		nullTime(t.CancelledAt.Valid, t.CancelledAt.Time),
		strconv.Itoa(t.SkipCount),
		SafeCell(t.CounterName.String),
		SafeCell(t.Note.String),
	}
}
