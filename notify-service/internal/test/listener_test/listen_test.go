package listener_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"

	"qms/notify-service/internal/listener"
	"qms/notify-service/internal/model"
)

// reply is the fake server's answer to one simple-protocol query. hangup closes the connection afterwards.
type reply struct {
	msgs   []pgproto3.BackendMessage
	hangup bool
}

// startFakePG serves just enough of the PostgreSQL wire protocol for LISTEN: each query is answered by the
// first entry in replies whose key is a substring of the SQL. It returns a DSN for the fake.
func startFakePG(t *testing.T, replies map[string]reply) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(c, replies)
		}
	}()
	return fmt.Sprintf("postgres://qms:qms@%s/qms?sslmode=disable&default_query_exec_mode=simple_protocol", ln.Addr())
}

func serve(c net.Conn, replies map[string]reply) {
	defer c.Close()
	be := pgproto3.NewBackend(c, c)
	if _, err := be.ReceiveStartupMessage(); err != nil {
		return
	}
	be.Send(&pgproto3.AuthenticationOk{})
	// pgx refuses simple-protocol queries unless the server reports these two settings.
	be.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	be.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if be.Flush() != nil {
		return
	}
	for {
		msg, err := be.Receive()
		if err != nil {
			return
		}
		q, ok := msg.(*pgproto3.Query)
		if !ok {
			return
		}
		r := answer(q.String, replies)
		for _, m := range r.msgs {
			be.Send(m)
		}
		be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		if be.Flush() != nil || r.hangup {
			return
		}
	}
}

func answer(sql string, replies map[string]reply) reply {
	for frag, r := range replies {
		if strings.Contains(sql, frag) {
			return r
		}
	}
	return reply{msgs: []pgproto3.BackendMessage{&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: "unexpected query: " + sql}}}
}

func TestListenHandlesNotificationsUntilConnectionDrops(t *testing.T) {
	dsn := startFakePG(t, map[string]reply{"LISTEN queue_events": {
		msgs: []pgproto3.BackendMessage{
			&pgproto3.CommandComplete{CommandTag: []byte("LISTEN")},
			&pgproto3.NotificationResponse{Channel: "queue_events", Payload: "not json"},
			&pgproto3.NotificationResponse{Channel: "queue_events", Payload: `{"kind":"called","queue_id":1,"ticket_id":2}`},
		},
		hangup: true,
	}})

	var got []model.Event
	var states []bool
	err := listener.Listen(context.Background(), dsn,
		func(e model.Event) { got = append(got, e) },
		func(up bool) { states = append(states, up) })
	if err == nil {
		t.Fatal("want error once the connection drops")
	}
	// The bad payload is skipped, the valid one is handed over, and LISTEN was reported active once.
	if len(got) != 1 || got[0].Kind != model.KindCalled || got[0].TicketID != 2 {
		t.Fatalf("events=%+v", got)
	}
	if len(states) != 1 || !states[0] {
		t.Fatalf("states=%v, want [true]", states)
	}
}

func TestListenWithNilStateCallback(t *testing.T) {
	dsn := startFakePG(t, map[string]reply{"LISTEN queue_events": {
		msgs: []pgproto3.BackendMessage{
			&pgproto3.CommandComplete{CommandTag: []byte("LISTEN")},
			&pgproto3.NotificationResponse{Channel: "queue_events", Payload: `{"kind":"recalled","queue_id":1,"ticket_id":5}`},
		},
		hangup: true,
	}})

	var got []model.Event
	if err := listener.Listen(context.Background(), dsn, func(e model.Event) { got = append(got, e) }, nil); err == nil {
		t.Fatal("want error once the connection drops")
	}
	if len(got) != 1 || got[0].TicketID != 5 {
		t.Fatalf("events=%+v", got)
	}
}

func TestListenConnectFailureIsReturned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens on this port any more
	dsn := fmt.Sprintf("postgres://qms:qms@%s/qms?sslmode=disable", addr)

	called := false
	err = listener.Listen(context.Background(), dsn, func(model.Event) { called = true }, nil)
	if err == nil || called {
		t.Fatalf("err=%v called=%v, want connect error and no events", err, called)
	}
}

func TestListenCommandFailureIsReturned(t *testing.T) {
	dsn := startFakePG(t, map[string]reply{"LISTEN queue_events": {msgs: []pgproto3.BackendMessage{
		&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42601", Message: "denied"},
	}}})

	var states []bool
	err := listener.Listen(context.Background(), dsn, func(model.Event) { t.Fatal("no events expected") },
		func(up bool) { states = append(states, up) })
	if err == nil {
		t.Fatal("want LISTEN error")
	}
	if len(states) != 0 {
		t.Fatalf("states=%v, want none before LISTEN succeeds", states)
	}
}
