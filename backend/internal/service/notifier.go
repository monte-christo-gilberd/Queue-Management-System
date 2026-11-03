package service

// EventNotifier dipenuhi oleh *ws.Hub secara struktural
// (Hub.Notify(queueID, action, payload)) sehingga tidak ada import cycle:
// service tidak import ws, ws tidak import service.
type EventNotifier interface {
	Notify(
		queueID int64,
		action string,
		payload interface{},
	)
}

// Nama event disamakan dengan internal/ws/message.go.
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

func notifyEvent(
	notifier EventNotifier,
	queueID int64,
	action string,
	payload interface{},
) {
	if notifier == nil {
		return
	}
	notifier.Notify(queueID, action, payload)
}
