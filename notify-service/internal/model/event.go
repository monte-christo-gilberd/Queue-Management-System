package model

// Jenis event yang dikirim trigger database lewat kolom kind.
const (
	// KindCalled: nomor dipanggil ke counter. Penerima: pemilik nomor dan tiket waiting terdepan.
	KindCalled = "called"
	// KindRecalled: owner memanggil ulang nomor yang sedang dilayani. Penerima: pemilik nomor.
	KindRecalled = "recalled"
	// KindSkipped: nomor dilewati permanen. Penerima: pemilik nomor.
	KindSkipped = "skipped"
	// KindRequeued: nomor kembali menunggu (skip dengan fallback, atau counter dihapus).
	KindRequeued = "requeued"
)

// Alasan untuk KindRequeued, dikirim di Event.Reason.
const (
	// ReasonSkip: nomor kembali karena dilewati dengan fallback.
	ReasonSkip = "skip"
	// ReasonCounterClosed: nomor kembali karena counter yang melayaninya dihapus.
	ReasonCounterClosed = "counter_closed"
)

// Event adalah satu payload NOTIFY yang sudah diurai dari JSON.
type Event struct {
	// Kind: jenis event (called, recalled, skipped, requeued).
	Kind string `json:"kind"`
	// QueueID: id antrian tempat kejadian ini terjadi.
	QueueID int64 `json:"queue_id"`
	// TicketID: id tiket yang terkait event ini.
	TicketID int64 `json:"ticket_id"`
	// Reason dan Counter hanya terisi untuk requeued. Counter dibawa di payload karena counter yang
	// dihapus sudah tidak bisa dibaca dari database.
	// Reason: alasan nomor kembali menunggu (ReasonSkip atau ReasonCounterClosed). Kosong untuk jenis lain.
	Reason string `json:"reason,omitempty"`
	// Counter: nama counter yang dihapus, dibawa di payload. Kosong untuk jenis lain.
	Counter string `json:"counter,omitempty"`
}
