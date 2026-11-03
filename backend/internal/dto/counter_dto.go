package dto

import "qms/internal/model"

// CounterServing: nomor yang sedang dilayani sebuah counter.
type CounterServing struct {
	TicketID     int64 `json:"ticket_id"`
	TicketNumber int64 `json:"ticket_number"`
}

// CounterResponse: satu counter antrian. Serving null bila counter sedang tidak melayani.
type CounterResponse struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	IsDefault bool            `json:"is_default"`
	Serving   *CounterServing `json:"serving"`
}

func NewCounterResponse(c model.CounterWithServing) CounterResponse {
	resp := CounterResponse{
		ID:        c.ID,
		Name:      c.Name,
		IsDefault: c.IsDefault,
	}
	if c.ServingTicketID.Valid && c.ServingNumber.Valid {
		resp.Serving = &CounterServing{
			TicketID:     c.ServingTicketID.Int64,
			TicketNumber: c.ServingNumber.Int64,
		}
	}
	return resp
}

// NewCounterFromModel: counter tanpa info nomor (hasil buat atau ganti nama).
func NewCounterFromModel(c model.Counter) CounterResponse {
	return CounterResponse{
		ID:        c.ID,
		Name:      c.Name,
		IsDefault: c.IsDefault,
	}
}
