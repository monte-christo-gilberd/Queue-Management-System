// Package export menulis riwayat antrian sebagai CSV atau XLSX (4.15).
// Keduanya ditulis bertahap (streaming) per halaman tiket, supaya antrian besar tidak dimuat sekaligus ke memori.
//
// Isinya:
//   - export.go: format berkas, judul kolom, label hasil, pengamanan sel (SafeCell), nama berkas (FileName), dan Sink.
//   - csv.go: Sink untuk CSV.
//   - xlsx.go: Sink untuk XLSX, memakai pustaka excelize.
//
// Package ini hanya memformat dan menulis data. Pengambilan data dari database dilakukan oleh repository dan service.
package export
