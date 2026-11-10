package dto

import "qms/internal/model"

// CounterServing: nomor yang sedang dilayani sebuah counter.
type CounterServing struct {
	// TicketID: id tiket yang sedang dilayani.
	TicketID int64 `json:"ticket_id"`
	// TicketNumber: nomor tiket yang sedang dilayani (tanpa prefix).
	TicketNumber int64 `json:"ticket_number"`
}

// CounterResponse: satu counter antrian. Serving null bila counter sedang tidak melayani.
type CounterResponse struct {
	// ID: id counter di database.
	ID int64 `json:"id"`
	// Name: nama counter.
	Name string `json:"name"`
	// IsDefault: true untuk counter default antrian. Counter default tidak bisa dihapus.
	IsDefault bool `json:"is_default"`
	// Serving: nomor yang sedang dilayani counter ini. Null bila tidak sedang melayani.
	Serving *CounterServing `json:"serving"`
}

// NewCounterResponse membuat CounterResponse dari model.CounterWithServing. Serving diisi hanya bila counter sedang
// melayani nomor (tiket dan nomornya sama-sama ada).
func NewCounterResponse(c model.CounterWithServing) CounterResponse {
	// Salin id, nama, dan status default; Serving diisi di bawah bila ada.
	resp := CounterResponse{
		ID:        c.ID,
		Name:      c.Name,
		IsDefault: c.IsDefault,
	}
	// Counter sedang melayani: id tiket dan nomornya sama-sama ada di database.
	if c.ServingTicketID.Valid && c.ServingNumber.Valid {
		// Buat CounterServing dari id tiket dan nomor yang sedang dilayani.
		resp.Serving = &CounterServing{
			TicketID:     c.ServingTicketID.Int64,
			TicketNumber: c.ServingNumber.Int64,
		}
	}
	return resp
}

// NewCounterFromModel: counter tanpa info nomor (hasil buat atau ganti nama).
func NewCounterFromModel(c model.Counter) CounterResponse {
	// Salin id, nama, dan status default. Serving selalu null karena model.Counter tidak memuat nomor yang dilayani.
	return CounterResponse{
		ID:        c.ID,
		Name:      c.Name,
		IsDefault: c.IsDefault,
	}
}
