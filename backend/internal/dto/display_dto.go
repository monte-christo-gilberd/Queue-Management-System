package dto

import (
	"qms/internal/model"
	"time"
)

// DisplayConfigResponse: konfigurasi layar untuk owner. Token membentuk URL layar (/display/<token>).
type DisplayConfigResponse struct {
	QueueID       int64      `json:"queue_id"`
	Token         string     `json:"token"`
	UpcomingCount int        `json:"upcoming_count"`
	SkippedCount  int        `json:"skipped_count"`
	CreatedAt     time.Time  `json:"created_at"`
	RotatedAt     *time.Time `json:"rotated_at"`
}

func NewDisplayConfigResponse(d model.QueueDisplay) DisplayConfigResponse {
	resp := DisplayConfigResponse{
		QueueID:       d.QueueID,
		Token:         d.Token,
		UpcomingCount: d.UpcomingCount,
		SkippedCount:  d.SkippedCount,
		CreatedAt:     d.CreatedAt,
	}
	if d.RotatedAt.Valid {
		t := d.RotatedAt.Time
		resp.RotatedAt = &t
	}
	return resp
}

// UpdateDisplayRequest: PUT /queues/:id/display. Jumlah yang tidak dikirim memakai nilai bawaan saat layar pertama
// kali diaktifkan, dan nilai yang sekarang bila layar sudah ada. Rentang diperiksa service.
type UpdateDisplayRequest struct {
	UpcomingCount *int `json:"upcoming_count"`
	SkippedCount  *int `json:"skipped_count"`
}

// DisplayCounterResponse: satu kartu counter di layar publik.
type DisplayCounterResponse struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Serving *string `json:"serving"`
}

// DisplaySnapshotResponse: respons PUBLIK (tanpa login). Hanya teks siap tampil. Jangan tambahkan id tiket, id user,
// owner_id, nama customer, atau catatan: tes kontrak mengunci daftar kunci ini.
type DisplaySnapshotResponse struct {
	QueueName    string                   `json:"queue_name"`
	QueueDesc    *string                  `json:"queue_desc"`
	Status       string                   `json:"status"`
	MultiCounter bool                     `json:"multi_counter"`
	Counters     []DisplayCounterResponse `json:"counters"`
	Upcoming     []string                 `json:"upcoming"`
	Skipped      []string                 `json:"skipped"`
	GeneratedAt  time.Time                `json:"generated_at"`
}

func NewDisplaySnapshotResponse(s model.DisplaySnapshot) DisplaySnapshotResponse {
	counters := make([]DisplayCounterResponse, 0, len(s.Counters))
	for _, c := range s.Counters {
		cr := DisplayCounterResponse{ID: c.ID, Name: c.Name}
		if c.Serving != "" {
			serving := c.Serving
			cr.Serving = &serving
		}
		counters = append(counters, cr)
	}
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
