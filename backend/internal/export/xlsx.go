package export

import (
	"fmt"
	"io"

	"qms/internal/model"

	"github.com/xuri/excelize/v2"
)

const sheetName = "Riwayat"

// xlsxDateFormat: format angka tanggal Excel yang setara dengan TimeLayout.
const xlsxDateFormat = "yyyy-mm-dd hh:mm:ss"

type xlsxSink struct {
	w         io.Writer
	f         *excelize.File
	sw        *excelize.StreamWriter
	prefix    string
	row       int
	dateStyle int
	closed    bool
}

// newXLSXSink memakai penulis streaming excelize: baris ditulis ke berkas sementara, bukan dikumpulkan di memori.
// Byte ke `w` baru keluar saat Close.
func newXLSXSink(w io.Writer, queue model.HistoryQueue) (*xlsxSink, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}
	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return nil, err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	numFmt := xlsxDateFormat
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})
	if err != nil {
		return nil, err
	}

	// Lebar kolom harus diatur sebelum baris pertama ditulis.
	widths := []float64{10, 22, 30, 16, 20, 20, 20, 20, 16, 16, 40}
	for i, width := range widths {
		if err := sw.SetColWidth(i+1, i+1, width); err != nil {
			return nil, err
		}
	}

	header := make([]interface{}, len(Headers))
	for i, h := range Headers {
		header[i] = excelize.Cell{StyleID: bold, Value: h}
	}
	if err := sw.SetRow("A1", header); err != nil {
		return nil, err
	}
	return &xlsxSink{w: w, f: f, sw: sw, prefix: queue.TicketPrefix, row: 2, dateStyle: dateStyle}, nil
}

func (s *xlsxSink) WriteTickets(tickets []model.HistoryTicket) error {
	for _, t := range tickets {
		cell, err := excelize.CoordinatesToCellName(1, s.row)
		if err != nil {
			return err
		}
		if err := s.sw.SetRow(cell, s.cells(t)); err != nil {
			return err
		}
		s.row++
	}
	return nil
}

func (s *xlsxSink) cells(t model.HistoryTicket) []interface{} {
	date := func(valid bool, v interface{}) interface{} {
		if !valid {
			return nil
		}
		return excelize.Cell{StyleID: s.dateStyle, Value: v}
	}
	return []interface{}{
		ticketLabel(s.prefix, t.TicketNumber),
		SafeCell(t.CustomerName),
		SafeCell(t.CustomerEmail),
		OutcomeLabel(t.Status),
		excelize.Cell{StyleID: s.dateStyle, Value: t.JoinedAt.UTC()},
		date(t.CalledAt.Valid, t.CalledAt.Time.UTC()),
		date(t.CompletedAt.Valid, t.CompletedAt.Time.UTC()),
		date(t.CancelledAt.Valid, t.CancelledAt.Time.UTC()),
		t.SkipCount,
		SafeCell(t.CounterName.String),
		SafeCell(t.Note.String),
	}
}

// Close menutup penulis streaming dan menulis seluruh berkas ke `w`. Aman dipanggil lebih dari sekali.
func (s *xlsxSink) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	defer s.f.Close()
	if err := s.sw.Flush(); err != nil {
		return fmt.Errorf("export: flush xlsx: %w", err)
	}
	return s.f.Write(s.w)
}
