package model

// QueueStatus: gambaran satu queue untuk pemegang tiket (atau owner).
type QueueStatus struct {
	Queue         Queue
	MyTicket      *Ticket // nil bila requester tidak punya tiket di queue ini
	CurrentNumber *int    // ticket_number completed terbaru; nil bila belum ada yang dilayani
	WaitingAhead  *int    // jumlah tiket waiting di depan tiket requester; nil bila tiket tidak sedang waiting
}
