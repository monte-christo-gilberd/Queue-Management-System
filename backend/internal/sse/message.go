package sse

import (
	"encoding/json"
	"time"
)

const (
	EventTicketJoined = "ticket_joined"
	EventTicketNext   = "ticket_next"
	EventQueueClosed  = "queue_closed"
	EventQueueUpdated = "queue_updated"
)

// Event adalah kontrak JSON ke browser web & Expo RN.
// Contoh: {"type":"ticket_joined","queue_id":1,"payload":{...},"ts":"..."}
type Event struct {
	Type      string      `json:"type"`
	QueueID   int64       `json:"queue_id"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"ts"`
}

func NewEvent(
	queueID int64,
	eventType string,
	payload interface{},
) Event {
	return Event{
		Type:      eventType,
		QueueID:   queueID,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
}

func MarshalEvent(
	queueID int64,
	eventType string,
	payload interface{},
) []byte {
	msg, err := json.Marshal(
		NewEvent(
			queueID,
			eventType,
			payload,
		),
	)
	if err != nil {
		return nil
	}
	return msg
}
