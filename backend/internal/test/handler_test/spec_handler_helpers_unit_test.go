package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"qms/internal/model"
	noHTMLValidator "qms/internal/validator"
	"reflect"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// specAny: pencocok argumen apa pun pada ekspektasi mock.
type specAnyArg struct{}

var specAny = specAnyArg{}

// specMatcher: pencocok argumen kustom pada ekspektasi mock.
type specMatcher func(v interface{}) bool

// specExpectation: satu ekspektasi (nama method + argumen) beserta nilai balik yang disiapkan.
type specExpectation struct {
	method string
	args   []interface{}
	rets   []interface{}
	hits   int
}

func (e *specExpectation) Return(rets ...interface{}) *specExpectation {
	e.rets = rets
	return e
}

// specQueueSvc: mock tulis-tangan untuk handler.QueueService. Setiap panggilan harus cocok
// dengan ekspektasi yang sudah dipasang lewat On(...); panggilan tak terduga membuat test gagal.
type specQueueSvc struct {
	t            *testing.T
	expectations []*specExpectation
	calls        map[string]int
}

func newSpecQueueSvc(t *testing.T) *specQueueSvc {
	return &specQueueSvc{t: t, calls: map[string]int{}}
}

func (m *specQueueSvc) On(method string, args ...interface{}) *specExpectation {
	e := &specExpectation{method: method, args: args}
	m.expectations = append(m.expectations, e)
	return e
}

func specArgMatches(want, got interface{}) bool {
	switch w := want.(type) {
	case specAnyArg:
		return true
	case specMatcher:
		return w(got)
	}
	return reflect.DeepEqual(want, got)
}

func (m *specQueueSvc) call(method string, args ...interface{}) []interface{} {
	m.calls[method]++
	for _, e := range m.expectations {
		if e.method != method || len(e.args) != len(args) {
			continue
		}
		ok := true
		for i := range args {
			if !specArgMatches(e.args[i], args[i]) {
				ok = false
				break
			}
		}
		if ok {
			e.hits++
			return e.rets
		}
	}
	m.t.Fatalf("mock: panggilan tak terduga %s%v", method, args)
	return nil
}

// count: jumlah panggilan ke method tertentu (termasuk yang tidak cocok).
func (m *specQueueSvc) count(method string) int {
	return m.calls[method]
}

// verify: setiap ekspektasi harus terpenuhi minimal sekali.
func (m *specQueueSvc) verify() {
	m.t.Helper()
	for _, e := range m.expectations {
		if e.hits == 0 {
			m.t.Errorf("mock: ekspektasi %s%v tidak terpenuhi", e.method, e.args)
		}
	}
}

func specRet(rets []interface{}, i int) interface{} {
	if i < len(rets) {
		return rets[i]
	}
	return nil
}

func specErr(rets []interface{}, i int) error {
	v := specRet(rets, i)
	if v == nil {
		return nil
	}
	return v.(error)
}

func (m *specQueueSvc) CreateQueue(_ context.Context, ownerID int64, name, desc string) (model.Queue, error) {
	r := m.call("CreateQueue", ownerID, name, desc)
	return specRet(r, 0).(model.Queue), specErr(r, 1)
}

func (m *specQueueSvc) ViewQueue(_ context.Context, queueID, requesterID int64) (model.Queue, bool, error) {
	r := m.call("ViewQueue", queueID, requesterID)
	return specRet(r, 0).(model.Queue), specRet(r, 1).(bool), specErr(r, 2)
}

func (m *specQueueSvc) ListQueue(_ context.Context, requesterID int64) ([]model.Queue, error) {
	r := m.call("ListQueue", requesterID)
	list, _ := specRet(r, 0).([]model.Queue)
	return list, specErr(r, 1)
}

func (m *specQueueSvc) UpdateQueue(_ context.Context, queueID, requesterID int64, name, desc string) error {
	r := m.call("UpdateQueue", queueID, requesterID, name, desc)
	return specErr(r, 0)
}

func (m *specQueueSvc) CloseQueue(_ context.Context, queueID, requesterID int64) error {
	r := m.call("CloseQueue", queueID, requesterID)
	return specErr(r, 0)
}

// specTicketSvc: stub untuk handler.TicketService. Mengembalikan nilai yang sudah ditentukan dan mencatat argumen.
type specTicketSvc struct {
	mu sync.Mutex

	ticket     model.Ticket
	status     model.QueueStatus
	tickets    []model.Ticket
	err        error
	gotQueue   int64
	gotUser    int64
	gotStatus  []string
	calls      int
	gotToken   string
	joinQueue  model.Queue
	resolveErr error
	joinErr    error
}

func (s *specTicketSvc) record(queueID, requesterID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.gotQueue = queueID
	s.gotUser = requesterID
}

func (s *specTicketSvc) JoinQueue(_ context.Context, queueID, userID int64) (model.Ticket, error) {
	s.record(queueID, userID)
	return s.ticket, s.joinErr
}

func (s *specTicketSvc) NextTicket(_ context.Context, queueID, requesterID int64, counterID int64) (model.Ticket, error) {
	s.record(queueID, requesterID)
	return s.ticket, s.err
}

func (s *specTicketSvc) ListTickets(_ context.Context, queueID, requesterID int64, statuses []string) ([]model.Ticket, error) {
	s.record(queueID, requesterID)
	s.gotStatus = statuses
	return s.tickets, s.err
}

func (s *specTicketSvc) QueueStatus(_ context.Context, queueID, requesterID int64) (model.QueueStatus, error) {
	s.record(queueID, requesterID)
	return s.status, s.err
}

func (s *specTicketSvc) ListMyTickets(_ context.Context, userID int64) ([]model.UserTicket, error) {
	s.record(0, userID)
	return nil, s.err
}

// ResolveJoinToken: stub verifikasi token join (token -> antrian).
func (s *specTicketSvc) ResolveJoinToken(_ context.Context, token string) (model.Queue, error) {
	s.gotToken = token
	return s.joinQueue, s.resolveErr
}

// specUserSvc: stub untuk handler.UserService.
type specUserSvc struct {
	token   string
	err     error
	gotID   string
	gotPass string
	calls   int
}

func (s *specUserSvc) Register(_ context.Context, email, username, password string) (string, error) {
	s.calls++
	s.gotID = email + "|" + username
	s.gotPass = password
	return s.token, s.err
}

func (s *specUserSvc) Login(_ context.Context, identifier, password string) (string, error) {
	s.calls++
	s.gotID = identifier
	s.gotPass = password
	return s.token, s.err
}

func (s *specUserSvc) Profile(_ context.Context, userID int64) (model.User, bool, error) {
	return model.User{ID: userID}, false, s.err
}

// specEngine: router uji dengan user_id dipasang seperti middleware auth.
func specEngine(userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	registerValidators.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			_ = v.RegisterValidation("no_html_tags", noHTMLValidator.NoHTMLTags)
		}
	})
	e := gin.New()
	e.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	return e
}

func specRequest(e *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	e.ServeHTTP(w, req)
	return w
}

func specDecode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("respons bukan JSON: %v (%s)", err, w.Body.String())
	}
	return out
}

// specErrorCode: kode error dari amplop {"error":{"code":...}}; "" bila tidak ada.
func specErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	body := specDecode(t, w)
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		return ""
	}
	code, _ := errObj["code"].(string)
	return code
}
