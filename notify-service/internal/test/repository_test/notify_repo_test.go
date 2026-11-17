package repository_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"qms/notify-service/internal/model"
	"qms/notify-service/internal/repository"
)

// Column type OIDs used by the fake results.
const (
	oidInt8 = 20
	oidInt4 = 23
	oidText = 25
)

// reply is the fake server's answer to one simple-protocol query.
type reply struct {
	msgs   []pgproto3.BackendMessage
	hangup bool
}

// startFakePG serves the PostgreSQL wire protocol on a loopback port. Each query is answered by the first
// entry in replies whose key is a substring of the SQL; anything else gets an error. It returns a DSN.
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
		if !ok { // Terminate or anything else: end the session.
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
	return errReply("unexpected query: " + sql)
}

func errReply(msg string) reply {
	return reply{msgs: []pgproto3.BackendMessage{&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: msg}}}
}

type col struct {
	name string
	oid  uint32
}

// rowsReply answers a SELECT. A nil cell is SQL NULL.
func rowsReply(cols []col, rows [][]*string) reply {
	fields := make([]pgproto3.FieldDescription, len(cols))
	for i, c := range cols {
		fields[i] = pgproto3.FieldDescription{Name: []byte(c.name), DataTypeOID: c.oid, DataTypeSize: -1}
	}
	msgs := []pgproto3.BackendMessage{&pgproto3.RowDescription{Fields: fields}}
	for _, row := range rows {
		vals := make([][]byte, len(row))
		for i, v := range row {
			if v != nil {
				vals[i] = []byte(*v)
			}
		}
		msgs = append(msgs, &pgproto3.DataRow{Values: vals})
	}
	msgs = append(msgs, &pgproto3.CommandComplete{CommandTag: fmt.Appendf(nil, "SELECT %d", len(rows))})
	return reply{msgs: msgs}
}

var ticketCols = []col{
	{"id", oidInt8}, {"queue_id", oidInt8}, {"ticket_number", oidInt4}, {"user_id", oidInt8},
	{"status", oidText}, {"queue_name", oidText}, {"ticket_prefix", oidText}, {"counter", oidText},
}

var firstCols = ticketCols[:7]

var subCols = []col{{"endpoint", oidText}, {"p256dh", oidText}, {"auth", oidText}}

// Distinct SQL fragments that identify each store query.
const (
	qTicket     = "THEN cc.name END"
	qPosition   = "o.sort_key < t.sort_key"
	qFirst      = "ORDER BY t.sort_key ASC LIMIT 1"
	qSubs       = "SELECT endpoint, p256dh, auth"
	qDeleteSubs = "DELETE FROM push_subscriptions"
	qDevices    = "FROM push_devices"
	qDeleteDevs = "DELETE FROM push_devices"
)

func newStore(t *testing.T, replies map[string]reply) (*repository.NotifyRepository, *sqlx.DB) {
	t.Helper()
	db, err := sqlx.Open("pgx", startFakePG(t, replies))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return repository.NewNotifyRepository(db), db
}

func TestTicketReadsCounterName(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qTicket: rowsReply(ticketCols, [][]*string{
		{new("7"), new("3"), new("12"), new("10"), new("called"), new("Poli"), new("B"), new("Counter 2")},
	})})
	tk, err := st.Ticket(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if tk.ID != 7 || tk.QueueID != 3 || tk.Number != 12 || tk.UserID != 10 || tk.Status != "called" ||
		tk.QueueName != "Poli" || tk.Prefix != "B" || tk.CounterName != "Counter 2" {
		t.Fatalf("ticket=%+v", tk)
	}
}

func TestTicketNullCounterLeavesNameEmpty(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qTicket: rowsReply(ticketCols, [][]*string{
		{new("7"), new("3"), new("12"), new("10"), new("waiting"), new("Poli"), new("A"), nil},
	})})
	tk, err := st.Ticket(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if tk.CounterName != "" || tk.Prefix != "A" {
		t.Fatalf("ticket=%+v", tk)
	}
}

func TestTicketMissingIsError(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qTicket: rowsReply(ticketCols, nil)})
	if _, err := st.Ticket(context.Background(), 99); err == nil {
		t.Fatal("want error for missing ticket")
	}
}

func TestTicketQueryErrorIsReturned(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qTicket: errReply("boom")})
	if _, err := st.Ticket(context.Background(), 1); err == nil {
		t.Fatal("want query error")
	}
}

func TestPositionOf(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qPosition: rowsReply([]col{{"pos", oidInt8}}, [][]*string{{new("4")}})})
	pos, err := st.PositionOf(context.Background(), 1)
	if err != nil || pos != 4 {
		t.Fatalf("pos=%d err=%v", pos, err)
	}
}

func TestPositionOfQueryError(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qPosition: errReply("boom")})
	if _, err := st.PositionOf(context.Background(), 1); err == nil {
		t.Fatal("want query error")
	}
}

func TestFirstWaitingReturnsTicket(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qFirst: rowsReply(firstCols, [][]*string{
		{new("5"), new("3"), new("8"), new("20"), new("waiting"), new("Poli"), new("A")},
	})})
	tk, err := st.FirstWaiting(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if tk == nil || tk.ID != 5 || tk.Number != 8 || tk.UserID != 20 || tk.Status != "waiting" || tk.QueueName != "Poli" {
		t.Fatalf("ticket=%+v", tk)
	}
}

func TestFirstWaitingNoneIsNilWithoutError(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qFirst: rowsReply(firstCols, nil)})
	tk, err := st.FirstWaiting(context.Background(), 3)
	if err != nil || tk != nil {
		t.Fatalf("tk=%+v err=%v, want nil, nil", tk, err)
	}
}

func TestFirstWaitingQueryErrorIsReturned(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qFirst: errReply("boom")})
	tk, err := st.FirstWaiting(context.Background(), 3)
	if err == nil || tk != nil {
		t.Fatalf("tk=%+v err=%v, want error", tk, err)
	}
}

func TestSubscriptionsReadsAllRows(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qSubs: rowsReply(subCols, [][]*string{
		{new("https://push/a"), new("k1"), new("a1")},
		{new("https://push/b"), new("k2"), new("a2")},
	})})
	subs, err := st.Subscriptions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 || subs[0].Endpoint != "https://push/a" || subs[1].P256dh != "k2" || subs[1].Auth != "a2" {
		t.Fatalf("subs=%+v", subs)
	}
}

func TestSubscriptionsEmpty(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qSubs: rowsReply(subCols, nil)})
	subs, err := st.Subscriptions(context.Background(), 10)
	if err != nil || len(subs) != 0 {
		t.Fatalf("subs=%+v err=%v", subs, err)
	}
}

func TestSubscriptionsQueryErrorIsReturned(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qSubs: errReply("boom")})
	if _, err := st.Subscriptions(context.Background(), 10); err == nil {
		t.Fatal("want query error")
	}
}

func TestSubscriptionsScanErrorIsReturned(t *testing.T) {
	// Two columns cannot fill the three destinations, so the row scan fails.
	st, _ := newStore(t, map[string]reply{qSubs: rowsReply(subCols[:2], [][]*string{{new("https://push/a"), new("k1")}})})
	if _, err := st.Subscriptions(context.Background(), 10); err == nil {
		t.Fatal("want scan error")
	}
}

func TestDeleteSubscription(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qDeleteSubs: {msgs: []pgproto3.BackendMessage{&pgproto3.CommandComplete{CommandTag: []byte("DELETE 1")}}}})
	if err := st.DeleteSubscription(context.Background(), model.Subscription{Endpoint: "https://push/a"}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSubscriptionErrorIsReturned(t *testing.T) {
	st, _ := newStore(t, map[string]reply{qDeleteSubs: errReply("boom")})
	if err := st.DeleteSubscription(context.Background(), model.Subscription{Endpoint: "https://push/a"}); err == nil {
		t.Fatal("want exec error")
	}
}

// FCM mati (EnableDevices tidak dipanggil): push_devices tidak di-query sama sekali.
func TestSubscriptionsSkipsDevicesWhenDisabled(t *testing.T) {
	// Tidak ada jawaban untuk push_devices: query ke sana akan gagal, jadi hasil sukses membuktikan tidak di-query.
	st, _ := newStore(t, map[string]reply{qSubs: rowsReply(subCols, [][]*string{{new("https://push/a"), new("k"), new("a")}})})
	subs, err := st.Subscriptions(context.Background(), 10)
	if err != nil || len(subs) != 1 || subs[0].Kind != model.KindWebPush {
		t.Fatalf("subs=%+v err=%v", subs, err)
	}
}

// FCM aktif: langganan Web Push lalu token FCM, masing-masing dengan Kind yang benar.
func TestSubscriptionsIncludesDevicesWhenEnabled(t *testing.T) {
	st, _ := newStore(t, map[string]reply{
		qSubs:    rowsReply(subCols, [][]*string{{new("https://push/a"), new("k"), new("a")}}),
		qDevices: rowsReply([]col{{"token", oidText}}, [][]*string{{new("tok-1")}, {new("tok-2")}}),
	})
	st.EnableDevices()
	subs, err := st.Subscriptions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 || subs[0].Endpoint != "https://push/a" ||
		subs[1].Kind != model.KindFCM || subs[1].Token != "tok-1" || subs[2].Token != "tok-2" {
		t.Fatalf("subs=%+v", subs)
	}
}

// Membaca push_devices gagal (mis. migrasi belum jalan): Web Push tetap dikembalikan tanpa galat.
func TestSubscriptionsDeviceErrorKeepsWebPush(t *testing.T) {
	st, _ := newStore(t, map[string]reply{
		qSubs:    rowsReply(subCols, [][]*string{{new("https://push/a"), new("k"), new("a")}}),
		qDevices: errReply("relation push_devices does not exist"),
	})
	st.EnableDevices()
	subs, err := st.Subscriptions(context.Background(), 10)
	if err != nil || len(subs) != 1 || subs[0].Endpoint != "https://push/a" {
		t.Fatalf("subs=%+v err=%v, want Web Push only", subs, err)
	}
}

// Token FCM yang mati dihapus dari push_devices menurut token, bukan dari push_subscriptions.
func TestDeleteSubscriptionFCMDeletesDevice(t *testing.T) {
	// Hanya push_devices yang dijawab; DELETE ke push_subscriptions akan gagal.
	st, _ := newStore(t, map[string]reply{qDeleteDevs: {msgs: []pgproto3.BackendMessage{&pgproto3.CommandComplete{CommandTag: []byte("DELETE 1")}}}})
	if err := st.DeleteSubscription(context.Background(), model.Subscription{Kind: model.KindFCM, Token: "tok-1"}); err != nil {
		t.Fatal(err)
	}
}
