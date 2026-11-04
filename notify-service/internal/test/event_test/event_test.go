package event_test

import (
	"testing"

	"qms/notify-service/internal/event"
)

func TestParseEvent(t *testing.T) {
	if _, err := event.Parse(`{"kind":"called","queue_id":1,"ticket_id":2}`); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`x`, `{}`, `{"kind":"called","queue_id":1}`} {
		if _, err := event.Parse(bad); err == nil {
			t.Fatalf("want error for %s", bad)
		}
	}
}

func TestParseRequeuedCarriesReasonAndCounter(t *testing.T) {
	e, err := event.Parse(`{"kind":"requeued","queue_id":1,"ticket_id":2,"reason":"counter_closed","counter":"Counter 2"}`)
	if err != nil || e.Reason != "counter_closed" || e.Counter != "Counter 2" {
		t.Fatalf("e=%+v err=%v", e, err)
	}
}
