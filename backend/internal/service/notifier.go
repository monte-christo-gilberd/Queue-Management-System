package service

// EventNotifier mengirim event real-time ke klien yang sedang terhubung ke antrian. Dipenuhi oleh *sse.Hub
// secara struktural (Hub.Notify(queueID, action, payload)), sehingga tidak ada import cycle:
// service tidak import sse, sse tidak import service.
type EventNotifier interface {
	Notify(
		queueID int64,
		action string,
		payload interface{},
	)
}

// Nama event disamakan dengan internal/sse/message.go. Dipakai sebagai argumen action di Notify, dan klien
// memakainya untuk memutuskan apa yang harus diperbarui di layar.
const (
	EventTicketJoined    = "ticket_joined"
	EventTicketNext      = "ticket_next"
	EventTicketSkipped   = "ticket_skipped"
	EventTicketCancelled = "ticket_cancelled"
	EventTicketRemoved   = "ticket_removed"
	EventCounterUpdated  = "counter_updated"
	EventQueueClosed     = "queue_closed"
	EventQueueUpdated    = "queue_updated"
	EventQueueQRUpdated  = "queue_qr_updated"
)

// notifyEvent mengirim event ke notifier bila terpasang. Notifier nil diabaikan, jadi service tetap berjalan
// tanpa SSE (mis. di tes).
func notifyEvent(
	notifier EventNotifier,
	queueID int64,
	action string,
	payload interface{},
) {
	// Notifier belum dipasang: tidak ada klien SSE yang bisa dikirimi event, jadi selesai tanpa melakukan apa-apa.
	if notifier == nil {
		return
	}
	// Teruskan event ke notifier (Hub SSE) untuk disebarkan ke klien yang terhubung ke antrian ini.
	notifier.Notify(queueID, action, payload)
}
