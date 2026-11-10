package model

import (
	"database/sql"
	"time"
)

// Counter: satu baris tabel counters, yaitu loket yang melayani nomor di sebuah antrian.
type Counter struct {
	// ID: id counter di database.
	ID int64 `db:"id"`
	// QueueID: id antrian tempat counter ini berada.
	QueueID int64 `db:"queue_id"`
	// Name: nama counter yang tampil di layar dan panel owner.
	Name string `db:"name"`
	// CreatedAt: waktu counter dibuat.
	CreatedAt time.Time `db:"created_at"`
	// IsDefault: true untuk counter default antrian. Counter default tidak bisa dihapus.
	IsDefault bool `db:"is_default"`
}

// CounterWithServing: counter beserta nomor yang sedang dilayaninya (LEFT JOIN tickets). Kosong bila tidak melayani.
type CounterWithServing struct {
	// Counter: data counter, disematkan langsung.
	Counter
	// ServingTicketID: id tiket yang sedang dilayani. Tidak valid (null) bila counter tidak melayani.
	ServingTicketID sql.NullInt64 `db:"serving_ticket_id"`
	// ServingNumber: nomor tiket yang sedang dilayani. Tidak valid (null) bila counter tidak melayani.
	ServingNumber sql.NullInt64 `db:"serving_number"`
}
