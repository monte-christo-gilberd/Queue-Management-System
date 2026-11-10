package export

import (
	"fmt"
	"io"

	"qms/internal/model"

	"github.com/xuri/excelize/v2"
)

// sheetName adalah nama lembar kerja (sheet) di dalam berkas XLSX.
const sheetName = "Riwayat"

// xlsxDateFormat: format angka tanggal Excel (number format) yang setara dengan TimeLayout.
const xlsxDateFormat = "yyyy-mm-dd hh:mm:ss"

// xlsxSink adalah Sink untuk XLSX: menerima tiket per halaman lalu menuliskannya sebagai baris Excel.
// Baris ditulis lewat StreamWriter dan seluruh berkas baru dikirim ke w saat Close.
type xlsxSink struct {
	// w adalah tujuan tulis akhir. Byte pertama baru ditulis saat Close.
	w io.Writer
	// f adalah berkas excelize yang dibangun di memori (dan di berkas sementara), lalu ditulis saat Close.
	f *excelize.File
	// sw adalah penulis streaming untuk sheet sheetName.
	sw *excelize.StreamWriter
	// prefix adalah awalan nomor tiket dari antrian.
	prefix string
	// row adalah nomor baris Excel yang akan ditulis berikutnya. Baris 1 berisi judul, jadi mulai dari 2.
	row int
	// dateStyle adalah ID gaya sel untuk kolom tanggal (memakai xlsxDateFormat).
	dateStyle int
	// closed menandai bahwa Close sudah dipanggil, supaya berkas tidak ditulis dua kali.
	closed bool
}

// newXLSXSink membuat Sink XLSX dan memakai penulis streaming excelize (pustaka Go untuk membaca dan menulis XLSX).
// Berkas XLSX sebenarnya arsip ZIP berisi XML. Dengan StreamWriter, baris ditulis ke berkas sementara,
// bukan dikumpulkan di memori. Byte ke `w` baru keluar saat Close.
func newXLSXSink(w io.Writer, queue model.HistoryQueue) (*xlsxSink, error) {
	// buat berkas XLSX kosong di excelize
	f := excelize.NewFile()
	// ganti nama sheet default "Sheet1" menjadi "Riwayat"; jika gagal, hentikan
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}
	// buat penulis streaming untuk sheet tersebut; jika gagal, hentikan
	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return nil, err
	}
	// gaya huruf tebal untuk baris judul; jika gagal, hentikan
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	// salin format tanggal ke variabel sendiri agar bisa diambil alamatnya
	numFmt := xlsxDateFormat
	// gaya sel tanggal dengan format yang ditentukan; jika gagal, hentikan
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})
	if err != nil {
		return nil, err
	}

	// Lebar kolom harus diatur sebelum baris pertama ditulis.
	widths := []float64{10, 22, 30, 16, 20, 20, 20, 20, 16, 16, 40}
	// atur lebar setiap kolom satu per satu; kolom dihitung dari 1, jadi i+1
	for i, width := range widths {
		// jika gagal mengatur lebar kolom, hentikan
		if err := sw.SetColWidth(i+1, i+1, width); err != nil {
			return nil, err
		}
	}

	// siapkan sel judul, setiap sel diberi gaya tebal
	header := make([]interface{}, len(Headers))
	// isi setiap sel dengan teks judul dari Headers
	for i, h := range Headers {
		header[i] = excelize.Cell{StyleID: bold, Value: h}
	}
	// tulis baris judul di A1; jika gagal, hentikan
	if err := sw.SetRow("A1", header); err != nil {
		return nil, err
	}
	// kembalikan sink dengan baris data pertama di baris 2 (baris 1 sudah berisi judul)
	return &xlsxSink{w: w, f: f, sw: sw, prefix: queue.TicketPrefix, row: 2, dateStyle: dateStyle}, nil
}

// WriteTickets menulis satu baris per tiket ke StreamWriter, mulai dari baris s.row, lalu menaikkan s.row.
func (s *xlsxSink) WriteTickets(tickets []model.HistoryTicket) error {
	// proses setiap tiket di halaman ini
	for _, t := range tickets {
		// ubah nomor baris s.row menjadi nama sel, misalnya A2; jika gagal, hentikan
		cell, err := excelize.CoordinatesToCellName(1, s.row)
		if err != nil {
			return err
		}
		// tulis seluruh kolom tiket ke baris tersebut; jika gagal, hentikan
		if err := s.sw.SetRow(cell, s.cells(t)); err != nil {
			return err
		}
		// lanjutkan ke baris berikutnya untuk tiket berikutnya
		s.row++
	}
	// semua tiket berhasil ditulis
	return nil
}

// cells mengubah satu tiket menjadi daftar sel sesuai urutan Headers. Kolom tanggal memakai gaya tanggal dan
// dikosongkan bila waktunya tidak ada (valid false).
func (s *xlsxSink) cells(t model.HistoryTicket) []interface{} {
	// date membuat sel tanggal dengan gaya tanggal; bila valid false, sel dibiarkan kosong (nil)
	date := func(valid bool, v interface{}) interface{} {
		// waktu tidak ada: kembalikan nil agar sel kosong
		if !valid {
			return nil
		}
		// waktu ada: bungkus dengan gaya tanggal
		return excelize.Cell{StyleID: s.dateStyle, Value: v}
	}
	// susun kolom sesuai urutan Headers
	return []interface{}{
		// nomor tiket, misalnya A-001
		ticketLabel(s.prefix, t.TicketNumber),
		// nama dan email pelanggan, diamankan dari formula injection
		SafeCell(t.CustomerName),
		SafeCell(t.CustomerEmail),
		// hasil tiket dalam bahasa Indonesia
		OutcomeLabel(t.Status),
		// waktu bergabung selalu ada, disimpan dalam UTC dengan gaya tanggal
		excelize.Cell{StyleID: s.dateStyle, Value: t.JoinedAt.UTC()},
		// waktu dipanggil, selesai, dan dibatalkan: kosong bila Valid false
		date(t.CalledAt.Valid, t.CalledAt.Time.UTC()),
		date(t.CompletedAt.Valid, t.CompletedAt.Time.UTC()),
		date(t.CancelledAt.Valid, t.CancelledAt.Time.UTC()),
		// jumlah lewati berupa angka
		t.SkipCount,
		// nama counter dan catatan, diamankan dari formula injection
		SafeCell(t.CounterName.String),
		SafeCell(t.Note.String),
	}
}

// Close menutup penulis streaming dan menulis seluruh berkas ke `w`. Aman dipanggil lebih dari sekali.
func (s *xlsxSink) Close() error {
	// sudah pernah ditutup: tidak perlu menulis lagi
	if s.closed {
		return nil
	}
	// tandai sudah ditutup agar pemanggilan berikutnya dilewati
	s.closed = true
	// bebaskan berkas excelize di memori/berkas sementara saat fungsi ini selesai
	defer s.f.Close()
	// selesaikan penulisan baris streaming; jika gagal, bungkus error dengan konteks
	if err := s.sw.Flush(); err != nil {
		return fmt.Errorf("export: flush xlsx: %w", err)
	}
	// tulis seluruh berkas XLSX ke w dan kembalikan hasilnya
	return s.f.Write(s.w)
}
