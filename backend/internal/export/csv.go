package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"qms/internal/model"
)

// utf8BOM membuat Excel membaca CSV sebagai UTF-8 (nama dan catatan berbahasa Indonesia tidak rusak).
const utf8BOM = "\xEF\xBB\xBF"

type csvSink struct {
	w       io.Writer
	cw      *csv.Writer
	prefix  string
	started bool
}

func newCSVSink(w io.Writer, queue model.HistoryQueue) *csvSink {
	cw := csv.NewWriter(w)
	cw.UseCRLF = true // Excel di Windows
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

func (s *csvSink) Close() error {
	if err := s.begin(); err != nil {
		return err
	}
	s.cw.Flush()
	return s.cw.Error()
}

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
