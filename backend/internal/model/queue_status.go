package model

// QueueStatus: gambaran satu queue untuk pemegang tiket (atau owner).
type QueueStatus struct {
	// Queue: data antrian yang dilihat.
	Queue Queue
	// MyTicket: tiket milik requester. Nil bila requester tidak punya tiket di queue ini.
	MyTicket *Ticket
	// CurrentNumber: ticket_number yang sedang dilayani (currently_served); nil bila belum ada.
	CurrentNumber *int
	// CurrentCounterName: nama counter yang memanggil nomor ini; nil bila counter default disembunyikan / belum ada.
	CurrentCounterName *string
	// WaitingAhead: jumlah tiket waiting di depan tiket requester; nil bila tiket tidak sedang waiting.
	WaitingAhead *int
	// PreviousNumber: ticket_number completed terbaru; nil bila belum ada.
	PreviousNumber *int
	// NextNumber: ticket_number waiting pertama; nil bila tidak ada yang menunggu.
	NextNumber *int
	// Serving: semua counter yang sedang melayani, yang paling baru dipanggil dulu.
	Serving []ServingEntry
}

// ServingEntry: satu counter yang sedang melayani nomor.
type ServingEntry struct {
	// CounterID: id counter yang sedang melayani.
	CounterID int64
	// CounterName: nama counter; nil untuk counter default (disembunyikan).
	CounterName *string
	// TicketNumber: nomor yang sedang dilayani counter ini.
	TicketNumber int
}

// UserTicket: tiket milik satu user beserta info antriannya (untuk daftar "tiket saya").
type UserTicket struct {
	// Ticket: data tiket, disematkan langsung.
	Ticket
	// QueueName: nama antrian tempat tiket ini berada.
	QueueName string `db:"queue_name"`
	// TicketPrefix: prefix nomor antrian, mis. "A" di "A-007".
	TicketPrefix string `db:"ticket_prefix"`
	// QueueStatus: status antrian: active atau closed.
	QueueStatus string `db:"queue_status"`
	// CurrentNumber: ticket_number yang sedang dilayani di antrian ini (completed terbaru); nil bila belum ada.
	CurrentNumber *int `db:"current_number"`
}
