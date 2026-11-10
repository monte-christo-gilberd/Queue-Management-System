package dto

import (
	"qms/internal/model"
	"time"
)

// DisplayConfigResponse: konfigurasi layar untuk owner. Token membentuk URL layar (/display/<token>).
type DisplayConfigResponse struct {
	// QueueID: id antrian yang ditampilkan layar ini.
	QueueID int64 `json:"queue_id"`
	// Token: bagian URL layar (/display/<token>). Berubah bila owner merotasi token.
	Token string `json:"token"`
	// UpcomingCount: berapa nomor "berikutnya" yang ditampilkan di layar (0 sampai 10).
	UpcomingCount int `json:"upcoming_count"`
	// SkippedCount: berapa nomor "dilewati" yang ditampilkan di layar (0 sampai 10).
	SkippedCount int `json:"skipped_count"`
	// CreatedAt: waktu layar pertama kali diaktifkan.
	CreatedAt time.Time `json:"created_at"`
	// RotatedAt: waktu token terakhir diganti. Null bila belum pernah diganti.
	RotatedAt *time.Time `json:"rotated_at"`
}

// NewDisplayConfigResponse membuat DisplayConfigResponse dari model.QueueDisplay. RotatedAt null bila token belum
// pernah diganti.
func NewDisplayConfigResponse(d model.QueueDisplay) DisplayConfigResponse {
	// Salin field konfigurasi; RotatedAt diisi terpisah di bawah karena nullable.
	resp := DisplayConfigResponse{
		QueueID:       d.QueueID,
		Token:         d.Token,
		UpcomingCount: d.UpcomingCount,
		SkippedCount:  d.SkippedCount,
		CreatedAt:     d.CreatedAt,
	}
	// Token pernah diganti (waktu rotasi ada): simpan alamat waktunya.
	if d.RotatedAt.Valid {
		// Salin waktu ke variabel baru agar pointer tidak menunjuk ke field model.
		t := d.RotatedAt.Time
		resp.RotatedAt = &t
	}
	return resp
}

// UpdateDisplayRequest: PUT /queues/:id/display. Jumlah yang tidak dikirim memakai nilai bawaan saat layar pertama
// kali diaktifkan, dan nilai yang sekarang bila layar sudah ada. Rentang diperiksa service.
type UpdateDisplayRequest struct {
	// UpcomingCount: jumlah nomor "berikutnya" yang diinginkan. Nil = tidak diubah.
	UpcomingCount *int `json:"upcoming_count"`
	// SkippedCount: jumlah nomor "dilewati" yang diinginkan. Nil = tidak diubah.
	SkippedCount *int `json:"skipped_count"`
}

// DisplayCounterResponse: satu kartu counter di layar publik.
type DisplayCounterResponse struct {
	// ID: id counter di database.
	ID int64 `json:"id"`
	// Name: nama counter yang tampil di layar.
	Name string `json:"name"`
	// Serving: teks nomor yang sedang dilayani counter ini (mis. "A-007"). Null bila counter sedang tidak melayani.
	Serving *string `json:"serving"`
}

// DisplaySnapshotResponse: respons PUBLIK (tanpa login). Hanya teks siap tampil. Jangan tambahkan id tiket, id user,
// owner_id, nama customer, atau catatan: tes kontrak mengunci daftar kunci ini.
type DisplaySnapshotResponse struct {
	// QueueName: nama antrian yang tampil di layar.
	QueueName string `json:"queue_name"`
	// QueueDesc: deskripsi antrian. Null bila tidak diisi.
	QueueDesc *string `json:"queue_desc"`
	// Status: status antrian: active atau closed.
	Status string `json:"status"`
	// MultiCounter: true bila antrian punya lebih dari satu counter (layar menampilkan kartu counter).
	MultiCounter bool `json:"multi_counter"`
	// Counters: daftar kartu counter.
	Counters []DisplayCounterResponse `json:"counters"`
	// Upcoming: label nomor yang akan dipanggil berikutnya, mis. "A-008".
	Upcoming []string `json:"upcoming"`
	// Skipped: label nomor yang dilewati, mis. "A-003".
	Skipped []string `json:"skipped"`
	// GeneratedAt: waktu snapshot ini dibuat, dipakai klien untuk tahu seberapa baru datanya.
	GeneratedAt time.Time `json:"generated_at"`
}

// NewDisplaySnapshotResponse membuat DisplaySnapshotResponse dari model.DisplaySnapshot. Setiap counter yang tidak
// sedang melayani dikirim dengan Serving null.
func NewDisplaySnapshotResponse(s model.DisplaySnapshot) DisplaySnapshotResponse {
	// Siapkan slice counter kosong dengan kapasitas sesuai jumlah counter.
	counters := make([]DisplayCounterResponse, 0, len(s.Counters))
	// Ubah setiap counter ke kartu layar.
	for _, c := range s.Counters {
		// Salin id dan nama counter; Serving diisi di bawah bila counter sedang melayani.
		cr := DisplayCounterResponse{ID: c.ID, Name: c.Name}
		// Counter sedang melayani nomor (teks tidak kosong): simpan alamat teksnya.
		if c.Serving != "" {
			// Salin teks ke variabel baru agar pointer tidak menunjuk ke field model.
			serving := c.Serving
			cr.Serving = &serving
		}
		// Tambahkan kartu counter ke hasil.
		counters = append(counters, cr)
	}
	// Susun respons; daftar counter yang sudah diubah dipakai, dan field lain disalin apa adanya.
	return DisplaySnapshotResponse{
		QueueName:    s.QueueName,
		QueueDesc:    s.QueueDesc,
		Status:       s.Status,
		MultiCounter: s.MultiCounter,
		Counters:     counters,
		Upcoming:     s.Upcoming,
		Skipped:      s.Skipped,
		GeneratedAt:  s.GeneratedAt,
	}
}
