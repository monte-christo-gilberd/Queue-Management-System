package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"qms/internal/sse"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDisplaySvc struct {
	snap      model.DisplaySnapshot
	snapErr   error
	queueID   int64
	cfg       model.QueueDisplay
	cfgErr    error
	gotUpc    int
	gotSkp    int
	configure int
}

func (s *stubDisplaySvc) Config(context.Context, int64, int64) (model.QueueDisplay, error) {
	return s.cfg, s.cfgErr
}
func (s *stubDisplaySvc) Configure(_ context.Context, id, _ int64, up, sk int) (model.QueueDisplay, error) {
	s.configure++
	s.gotUpc, s.gotSkp = up, sk
	if up > 10 {
		return model.QueueDisplay{}, apperror.ErrInvalidDisplayConfig
	}
	return model.QueueDisplay{QueueID: id, Token: "tok", UpcomingCount: up, SkippedCount: sk}, nil
}
func (s *stubDisplaySvc) Rotate(context.Context, int64, int64) (model.QueueDisplay, error) {
	return model.QueueDisplay{Token: "new"}, s.cfgErr
}
func (s *stubDisplaySvc) Disable(context.Context, int64, int64) error { return s.cfgErr }
func (s *stubDisplaySvc) Snapshot(context.Context, string) (model.DisplaySnapshot, error) {
	return s.snap, s.snapErr
}
func (s *stubDisplaySvc) QueueIDForToken(context.Context, string) (int64, error) {
	return s.queueID, s.snapErr
}

func displayEngine(svc *stubDisplaySvc, hub *sse.Hub) *gin.Engine {
	e := specEngine(7)
	h := handler.NewDisplayHandler(svc, hub)
	e.GET("/display/:token", h.Snapshot)
	e.GET("/display/:token/event", h.Events)
	e.GET("/queues/:queue_id/display", h.Get)
	e.PUT("/queues/:queue_id/display", h.Update)
	e.POST("/queues/:queue_id/display/rotate", h.Rotate)
	e.DELETE("/queues/:queue_id/display", h.Disable)
	return e
}

// Tes kontrak: respons PUBLIK hanya boleh memuat kunci di daftar ini. Kunci baru yang ditambahkan tanpa sengaja
// (id tiket, id user, owner_id, nama customer, catatan) membuat tes ini gagal.
func TestDisplaySnapshot_PublicResponseKeysAreLocked(t *testing.T) {
	desc := "Pelayanan umum"
	svc := &stubDisplaySvc{snap: model.DisplaySnapshot{
		QueueName: "Loket A", QueueDesc: &desc, Status: "active", MultiCounter: true,
		Counters:    []model.DisplayCounter{{ID: 1, Name: "Counter 1", Serving: "A-001"}, {ID: 2, Name: "Cepat"}},
		Upcoming:    []string{"A-002"},
		Skipped:     []string{"A-000"},
		GeneratedAt: time.Now(),
	}}
	w := specRequest(displayEngine(svc, sse.NewHub()), http.MethodGet, "/display/abc", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	keys := make([]string, 0)
	for k := range body.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"counters", "generated_at", "multi_counter", "queue_desc", "queue_name", "skipped", "status", "upcoming"}, keys)

	var counters []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body.Data["counters"], &counters))
	require.Len(t, counters, 2)
	for _, c := range counters {
		ck := make([]string, 0)
		for k := range c {
			ck = append(ck, k)
		}
		sort.Strings(ck)
		assert.Equal(t, []string{"id", "name", "serving"}, ck)
	}
	assert.Equal(t, "null", string(counters[1]["serving"]), "counter kosong = serving null")
	for _, forbidden := range []string{"user_id", "owner_id", "ticket_id", "note", "sort_key"} {
		assert.NotContains(t, w.Body.String(), forbidden)
	}
}

func TestDisplaySnapshot_UnknownTokenIsGeneric404(t *testing.T) {
	svc := &stubDisplaySvc{snapErr: apperror.ErrNotFound}
	w := specRequest(displayEngine(svc, sse.NewHub()), http.MethodGet, "/display/whatever", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "NOT_FOUND", specErrorCode(t, w))
}

func TestDisplayUpdate_DefaultsKeepExistingAndValidate(t *testing.T) {
	// Belum ada layar: nilai bawaan 5 dan 0.
	svc := &stubDisplaySvc{cfgErr: apperror.ErrNotFound}
	e := displayEngine(svc, sse.NewHub())
	w := specRequest(e, http.MethodPut, "/queues/5/display", `{}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 5, svc.gotUpc)
	assert.Equal(t, 0, svc.gotSkp)

	// Sudah ada: yang tidak dikirim tetap.
	svc2 := &stubDisplaySvc{cfg: model.QueueDisplay{UpcomingCount: 3, SkippedCount: 4}}
	e2 := displayEngine(svc2, sse.NewHub())
	specRequest(e2, http.MethodPut, "/queues/5/display", `{"skipped_count":9}`)
	assert.Equal(t, 3, svc2.gotUpc)
	assert.Equal(t, 9, svc2.gotSkp)

	// 0 adalah nilai sah (bukan "tidak dikirim").
	specRequest(e2, http.MethodPut, "/queues/5/display", `{"upcoming_count":0}`)
	assert.Equal(t, 0, svc2.gotUpc)

	w = specRequest(e2, http.MethodPut, "/queues/5/display", `{"upcoming_count":11}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, http.StatusBadRequest, specRequest(e2, http.MethodPut, "/queues/abc/display", `{}`).Code)
}

func TestDisplayOwnerRoutes_NotOwnerIs404AndDisableIs204(t *testing.T) {
	svc := &stubDisplaySvc{cfgErr: apperror.ErrNotFound}
	e := displayEngine(svc, sse.NewHub())
	assert.Equal(t, http.StatusNotFound, specRequest(e, http.MethodGet, "/queues/5/display", "").Code)
	assert.Equal(t, http.StatusNotFound, specRequest(e, http.MethodPost, "/queues/5/display/rotate", "").Code)

	ok := displayEngine(&stubDisplaySvc{}, sse.NewHub())
	assert.Equal(t, http.StatusNoContent, specRequest(ok, http.MethodDelete, "/queues/5/display", "").Code)
}

func TestDisplayEvents_UnknownTokenIs404AndConnectionCapIs429(t *testing.T) {
	hub := sse.NewHub()
	go hub.Run()
	svc := &stubDisplaySvc{queueID: 5, snapErr: apperror.ErrNotFound}
	assert.Equal(t, http.StatusNotFound, specRequest(displayEngine(svc, hub), http.MethodGet, "/display/x/event", "").Code)

	svc.snapErr = nil
	e := displayEngine(svc, hub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Penuhi batas 20 sambungan layar untuk antrian 5.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/display/x/event", strings.NewReader("")).WithContext(ctx)
		go e.ServeHTTP(httptest.NewRecorder(), req)
	}
	require.Eventually(t, func() bool { return hub.RoomSize(sse.DisplayRoom(5)) == 20 }, 3*time.Second, 10*time.Millisecond)

	w := specRequest(e, http.MethodGet, "/display/x/event", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "TOO_MANY_DISPLAYS", specErrorCode(t, w))
}
