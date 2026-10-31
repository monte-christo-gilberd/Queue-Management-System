package message_test

import (
	"testing"

	"qms/notify-service/internal/message"
)

func TestLabel(t *testing.T) {
	if got := message.Label(7); got != "A-007" {
		t.Fatalf("got %s", got)
	}
}
