package model

// QueueStatus: gambaran satu queue untuk pemegang tiket (atau owner).
type QueueStatus struct {
	Queue          Queue
	MyTicket       *Ticket // nil bila requester tidak punya tiket di queue ini
	CurrentNumber  *int    // ticket_number completed terbaru; nil bila belum ada yang dilayani
	WaitingAhead   *int    // jumlah tiket waiting di depan tiket requester; nil bila tiket tidak sedang waiting
	PreviousNumber *int    // ticket_number completed sebelum CurrentNumber; nil bila belum ada
	NextNumber     *int    // ticket_number waiting pertama; nil bila tidak ada yang menunggu
}

// UserTicket: tiket milik satu user beserta info antriannya (untuk daftar "tiket saya").
type UserTicket struct {
	Ticket
	QueueName   string `db:"queue_name"`
	QueueStatus string `db:"queue_status"`
	// CurrentNumber: ticket_number yang sedang dilayani di antrian ini (completed terbaru); nil bila belum ada.
	CurrentNumber *int `db:"current_number"`
}
