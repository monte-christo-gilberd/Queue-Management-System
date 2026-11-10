package sse

import (
	"encoding/json"
	"time"
)

// Nama-nama tipe event. Nilainya dikirim apa adanya ke klien di field "type" dan dibaca oleh frontend.
const (
	// EventTicketJoined: ada pelanggan baru bergabung ke antrian.
	EventTicketJoined = "ticket_joined"
	// EventTicketNext: tiket berikutnya dipanggil untuk dilayani.
	EventTicketNext = "ticket_next"
	// EventTicketSkipped: tiket yang sedang dilayani dilewati.
	EventTicketSkipped = "ticket_skipped"
	// EventTicketCancelled: pelanggan membatalkan tiketnya.
	EventTicketCancelled = "ticket_cancelled"
	// EventTicketRemoved: tiket dihapus dari antrian.
	EventTicketRemoved = "ticket_removed"
	// EventCounterUpdated: data counter (loket) berubah.
	EventCounterUpdated = "counter_updated"
	// EventQueueClosed: antrian ditutup.
	EventQueueClosed = "queue_closed"
	// EventQueueUpdated: pengaturan antrian berubah.
	EventQueueUpdated = "queue_updated"
	// EventDisplayChanged: sinyal tanpa data untuk layar publik ("ambil ulang snapshot").
	EventDisplayChanged = "display_changed"
)

// Event adalah kontrak JSON ke browser web & Expo RN.
// Contoh: {"type":"ticket_joined","queue_id":1,"payload":{...},"ts":"..."}
type Event struct {
	// Type adalah nama event, misalnya "ticket_joined" (lihat konstanta Event... di atas).
	Type string `json:"type"`
	// QueueID adalah ID antrian tempat event terjadi.
	QueueID int64 `json:"queue_id"`
	// Payload adalah data tambahan event. Tidak ditulis ke JSON bila nil (omitempty).
	Payload interface{} `json:"payload,omitempty"`
	// Timestamp adalah waktu event dibuat dalam UTC, ditulis di field "ts".
	Timestamp time.Time `json:"ts"`
}

// NewEvent membuat Event dengan waktu sekarang dalam UTC.
func NewEvent(
	queueID int64,
	eventType string,
	payload interface{},
) Event {
	// isi semua field Event, waktu diambil saat fungsi ini dipanggil
	return Event{
		Type:      eventType,
		QueueID:   queueID,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
}

// MarshalEvent mengubah event menjadi JSON siap kirim. Mengembalikan nil bila JSON gagal dibuat.
func MarshalEvent(
	queueID int64,
	eventType string,
	payload interface{},
) []byte {
	// buat Event lalu ubah menjadi JSON
	msg, err := json.Marshal(
		NewEvent(
			queueID,
			eventType,
			payload,
		),
	)
	// gagal diubah menjadi JSON: kembalikan nil agar pemanggil tidak mengirim data rusak
	if err != nil {
		return nil
	}
	// JSON berhasil dibuat
	return msg
}
