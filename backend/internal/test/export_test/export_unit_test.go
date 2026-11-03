package export_test

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"qms/internal/export"
	"qms/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

var joined = time.Date(2026, 11, 3, 9, 15, 30, 0, time.UTC)

func ticket(n int, status, name string) model.HistoryTicket {
	return model.HistoryTicket{
		TicketID: int64(n), TicketNumber: n, Status: status, JoinedAt: joined,
		CustomerName: name, CustomerEmail: name + "@mail.com",
	}
}

func readCSV(t *testing.T, raw []byte) [][]string {
	t.Helper()
	require.True(t, bytes.HasPrefix(raw, []byte("\xEF\xBB\xBF")), "CSV diawali BOM UTF-8")
	records, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	require.NoError(t, err)
	return records
}

func TestParseFormat(t *testing.T) {
	f, ok := export.ParseFormat("csv")
	assert.True(t, ok)
	assert.Equal(t, export.CSV, f)
	f, ok = export.ParseFormat("xlsx")
	assert.True(t, ok)
	assert.Equal(t, export.XLSX, f)
	for _, bad := range []string{"", "CSV", "xls", "pdf", "csv "} {
		_, ok := export.ParseFormat(bad)
		assert.False(t, ok, bad)
	}
}

func TestSafeCell_BlocksFormulaInjection(t *testing.T) {
	for _, danger := range []string{"=1+1", "+cmd", "-2+3", "@SUM(A1)", "\tx", "\rx", `=HYPERLINK("http://x")`} {
		assert.Equal(t, "'"+danger, export.SafeCell(danger), danger)
	}
	for _, safe := range []string{"", "budi", "a=b", "12 Jan", "'sudah"} {
		assert.Equal(t, safe, export.SafeCell(safe), safe)
	}
}

func TestFileName(t *testing.T) {
	assert.Equal(t, "riwayat-loket-utama-54.csv", export.FileName("Loket Utama!", 54, export.CSV))
	assert.Equal(t, "riwayat-kasir-7.xlsx", export.FileName("  Kasir  ", 7, export.XLSX))
	assert.Equal(t, "riwayat-antrian-9.csv", export.FileName("日本語", 9, export.CSV), "tanpa huruf ASCII jatuh ke antrian")
	assert.Equal(t, "riwayat-antrian-1.csv", export.FileName("", 1, export.CSV))
	long := export.FileName(strings.Repeat("abc ", 40), 3, export.CSV)
	assert.LessOrEqual(t, len(long), len("riwayat-")+40+len("-3.csv"))
	for _, nasty := range []string{`../../etc/passwd`, "a\"b\r\nSet-Cookie: x", "a;b"} {
		name := export.FileName(nasty, 1, export.CSV)
		assert.NotContains(t, name, "/")
		assert.NotContains(t, name, `"`)
		assert.NotContains(t, name, "\n")
		assert.NotContains(t, name, ";")
	}
}

func TestOutcomeLabel(t *testing.T) {
	assert.Equal(t, "Selesai", export.OutcomeLabel("completed"))
	assert.Equal(t, "Dilewati", export.OutcomeLabel("skipped"))
	assert.Equal(t, "Dibatalkan", export.OutcomeLabel("cancelled"))
	assert.Equal(t, "Dihapus", export.OutcomeLabel("deleted"))
	assert.Equal(t, "Belum dilayani", export.OutcomeLabel("waiting"))
	assert.Equal(t, "Belum dilayani", export.OutcomeLabel("currently_served"))
}

func TestCSV_HeaderRowsAndEncoding(t *testing.T) {
	var buf bytes.Buffer
	sink, err := export.NewSink(export.CSV, &buf, model.HistoryQueue{TicketPrefix: "B"})
	require.NoError(t, err)

	done := ticket(1, "completed", "budi")
	done.CompletedAt = sql.NullTime{Time: joined.Add(10 * time.Minute), Valid: true}
	done.CounterName = sql.NullString{String: "Counter 2", Valid: true}
	done.Note = sql.NullString{String: "minta \"struk\", dan\nfaktur", Valid: true}
	require.NoError(t, sink.WriteTickets([]model.HistoryTicket{done, ticket(2, "waiting", "ani")}))
	require.NoError(t, sink.Close())

	rows := readCSV(t, buf.Bytes())
	require.Len(t, rows, 3)
	assert.Equal(t, export.Headers, rows[0])
	assert.Equal(t, []string{"B-001", "budi", "budi@mail.com", "Selesai", "2026-11-03 09:15:30", "", "2026-11-03 09:25:30", "", "0", "Counter 2", "minta \"struk\", dan\nfaktur"}, rows[1])
	assert.Equal(t, "Belum dilayani", rows[2][3])
	assert.Contains(t, buf.String(), "\r\n", "akhir baris CRLF untuk Excel")
}

func TestCSV_EmptyQueueHasHeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	sink, err := export.NewSink(export.CSV, &buf, model.HistoryQueue{})
	require.NoError(t, err)
	require.NoError(t, sink.Close())

	rows := readCSV(t, buf.Bytes())
	require.Len(t, rows, 1)
	assert.Equal(t, export.Headers, rows[0])
}

func TestCSV_DefaultPrefixAndInjectionGuard(t *testing.T) {
	var buf bytes.Buffer
	sink, _ := export.NewSink(export.CSV, &buf, model.HistoryQueue{})
	bad := ticket(7, "completed", "=cmd")
	bad.Note = sql.NullString{String: "@evil", Valid: true}
	bad.CounterName = sql.NullString{String: "+x", Valid: true}
	require.NoError(t, sink.WriteTickets([]model.HistoryTicket{bad}))
	require.NoError(t, sink.Close())

	row := readCSV(t, buf.Bytes())[1]
	assert.Equal(t, "A-007", row[0], "awalan kosong jatuh ke A")
	assert.Equal(t, "'=cmd", row[1])
	assert.Equal(t, "'+x", row[9])
	assert.Equal(t, "'@evil", row[10])
}

func TestCSV_NoBytesBeforeFirstWrite(t *testing.T) {
	var buf bytes.Buffer
	_, err := export.NewSink(export.CSV, &buf, model.HistoryQueue{})
	require.NoError(t, err)
	assert.Zero(t, buf.Len(), "header HTTP lazy bergantung pada ini: belum ada byte sebelum tulis pertama")
}

func TestXLSX_OpensWithHeaderRowsAndDates(t *testing.T) {
	var buf bytes.Buffer
	sink, err := export.NewSink(export.XLSX, &buf, model.HistoryQueue{TicketPrefix: "C"})
	require.NoError(t, err)

	first := ticket(1, "completed", "budi")
	first.CompletedAt = sql.NullTime{Time: joined.Add(10 * time.Minute), Valid: true}
	first.SkipCount = 2
	first.Note = sql.NullString{String: "=1+1", Valid: true}
	require.NoError(t, sink.WriteTickets([]model.HistoryTicket{first}))
	require.NoError(t, sink.WriteTickets([]model.HistoryTicket{ticket(2, "skipped", "ani")}))
	require.NoError(t, sink.Close())
	assert.True(t, bytes.HasPrefix(buf.Bytes(), []byte("PK")), "XLSX adalah zip")

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	rows, err := f.GetRows("Riwayat")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, export.Headers, rows[0])
	assert.Equal(t, "C-001", rows[1][0])
	assert.Equal(t, "Selesai", rows[1][3])
	assert.Equal(t, "2026-11-03 09:15:30", rows[1][4], "tanggal bergabung")
	assert.Equal(t, "", rows[1][5], "belum dipanggil: sel kosong")
	assert.Equal(t, "2026-11-03 09:25:30", rows[1][6], "tanggal selesai")
	assert.Equal(t, "2", rows[1][8], "jumlah dilewati sebagai angka")
	assert.Equal(t, "'=1+1", rows[1][10], "catatan berawalan = diberi pengaman")
	assert.Equal(t, "Dilewati", rows[2][3])
}

func TestXLSX_EmptyQueueHasHeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	sink, err := export.NewSink(export.XLSX, &buf, model.HistoryQueue{})
	require.NoError(t, err)
	require.NoError(t, sink.Close())
	require.NoError(t, sink.Close(), "Close kedua tidak error")

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	rows, _ := f.GetRows("Riwayat")
	require.Len(t, rows, 1)
}

func TestXLSX_ManyRowsStayOrdered(t *testing.T) {
	var buf bytes.Buffer
	sink, _ := export.NewSink(export.XLSX, &buf, model.HistoryQueue{})
	for page := 0; page < 3; page++ {
		batch := make([]model.HistoryTicket, 0, 500)
		for i := 1; i <= 500; i++ {
			batch = append(batch, ticket(page*500+i, "completed", "u"))
		}
		require.NoError(t, sink.WriteTickets(batch))
	}
	require.NoError(t, sink.Close())

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	rows, _ := f.GetRows("Riwayat")
	require.Len(t, rows, 1501)
	assert.Equal(t, "A-1500", rows[1500][0])
}

func TestNewSink_UnknownFormat(t *testing.T) {
	_, err := export.NewSink(export.Format("pdf"), &bytes.Buffer{}, model.HistoryQueue{})
	assert.Error(t, err)
}
