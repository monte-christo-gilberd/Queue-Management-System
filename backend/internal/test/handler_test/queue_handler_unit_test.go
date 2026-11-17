package handler_test

import (
	"database/sql"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const specOwner int64 = 7

func specQueueEngine(svc *specQueueSvc, userID int64) *gin.Engine {
	e := specEngine(userID)
	h := handler.NewQueueHandlerWithQR(svc, nil)
	e.POST("/queues", h.Create)
	e.GET("/queues", h.List)
	e.GET("/queues/:queue_id", h.Get)
	e.PUT("/queues/:queue_id", h.Update)
	e.POST("/queues/:queue_id/close", h.Close)
	e.GET("/queues/:queue_id/settings/skip", h.GetSkipSettings)
	e.PUT("/queues/:queue_id/settings/skip", h.UpdateSkipSettings)
	return e
}

func specOpenQueue(id int64) model.Queue {
	return model.Queue{
		ID:        id,
		OwnerID:   specOwner,
		QueueName: "Loket Utama",
		QueueDesc: sql.NullString{String: "Pelayanan pagi", Valid: true},
		Status:    "active",
		CreatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}
}

func TestSpec_CreateQueue_NewQueueReturns201WithNullClosedAt(t *testing.T) {
	// US-1 / 4.1 langkah 2: antrian baru dibuat dan terbuka; closed_at null; kunci snake_case.
	svc := newSpecQueueSvc(t)
	svc.On("CreateQueue", specOwner, "Loket Utama", "Pelayanan pagi").
		Return(specOpenQueue(1), nil)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPost, "/queues", `{"queue_name":"Loket Utama","queue_description":"Pelayanan pagi"}`)

	require.Equal(t, http.StatusCreated, w.Code)
	body := specDecode(t, w)
	assert.Equal(t, "create_queue", body["data"].(map[string]interface{})["action"])
	queue := body["data"].(map[string]interface{})["queue"].(map[string]interface{})
	assert.Equal(t, "Loket Utama", queue["queue_name"])
	assert.Equal(t, "Pelayanan pagi", queue["queue_desc"])
	assert.Equal(t, "active", queue["status"])
	assert.Equal(t, float64(specOwner), queue["owner_id"])
	assert.Contains(t, queue, "closed_at")
	assert.Nil(t, queue["closed_at"])
	svc.verify()
}

func TestSpec_CreateQueue_EmptyOrHTMLNameIsRejectedBeforeService(t *testing.T) {
	// 4.1 edge case (nama kosong) & TC-08-06 (nama berisi HTML): ditolak, service tidak dipanggil.
	cases := map[string]string{
		"empty name":   `{"queue_name":"","queue_description":""}`,
		"missing":      `{"queue_description":"x"}`,
		"html in name": `{"queue_name":"<script>alert(1)</script>"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := newSpecQueueSvc(t)
			e := specQueueEngine(svc, specOwner)

			w := specRequest(e, http.MethodPost, "/queues", body)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "BAD_REQUEST", specErrorCode(t, w))
			assert.Equal(t, 0, svc.count("CreateQueue"))
		})
	}
}

func TestSpec_CreateQueue_InactiveSubscriptionIs402(t *testing.T) {
	// US-21 AC1 / TC-04-01: owner tanpa langganan aktif diminta membayar (402) dan antrian tidak dibuat.
	svc := newSpecQueueSvc(t)
	svc.On("CreateQueue", specOwner, "Antrian", "").
		Return(model.Queue{}, apperror.ErrSubscriptionInactive)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPost, "/queues", `{"queue_name":"Antrian"}`)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Equal(t, "SUBSCRIPTION_INACTIVE", specErrorCode(t, w))
	assert.NotContains(t, w.Body.String(), `"queue":`)
}

func TestSpec_GetQueue_OwnerSeesOwnerID(t *testing.T) {
	// 4.1: owner melihat detail antrian beserta owner_id.
	svc := newSpecQueueSvc(t)
	svc.On("ViewQueue", int64(1), specOwner).Return(specOpenQueue(1), true, nil)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodGet, "/queues/1", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(specOwner), data["queue"].(map[string]interface{})["owner_id"])
}

func TestSpec_GetQueue_TicketHolderSeesPublicViewOnly(t *testing.T) {
	// 4.4 / TC-08-03: pemegang tiket hanya melihat info publik; owner_id tidak ikut.
	svc := newSpecQueueSvc(t)
	q := specOpenQueue(1)
	q.QueueDesc = sql.NullString{}
	svc.On("ViewQueue", int64(1), int64(55)).Return(q, false, nil)
	e := specQueueEngine(svc, 55)

	w := specRequest(e, http.MethodGet, "/queues/1", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	queue := data["queue"].(map[string]interface{})
	assert.NotContains(t, queue, "owner_id")
	assert.Nil(t, queue["queue_desc"], "deskripsi kosong harus null")
	assert.Equal(t, "Loket Utama", queue["queue_name"])
}

func TestSpec_GetQueue_OutsiderGetsNotFoundWithoutLeakingName(t *testing.T) {
	// TC-08-03: owner lain / orang asing -> 404 "Antrian tidak ditemukan", nama antrian tidak bocor.
	svc := newSpecQueueSvc(t)
	svc.On("ViewQueue", int64(1), int64(99)).Return(model.Queue{}, false, apperror.ErrNotFound)
	e := specQueueEngine(svc, 99)

	w := specRequest(e, http.MethodGet, "/queues/1", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "NOT_FOUND", specErrorCode(t, w))
	assert.False(t, strings.Contains(w.Body.String(), "Loket"))
}

func TestSpec_UpdateQueue_OwnerGetsSuccess(t *testing.T) {
	// US-2 AC1: owner mengubah nama dan deskripsi.
	svc := newSpecQueueSvc(t)
	svc.On("UpdateQueue", int64(1), specOwner, "Loket Baru", "Jam 08.00").Return(nil)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPut, "/queues/1", `{"queue_name":"Loket Baru","queue_description":"Jam 08.00"}`)

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "success", data["status"])
	svc.verify()
}

func TestSpec_UpdateQueue_InactiveSubscriptionIs402(t *testing.T) {
	// Change log 2026-10-02 / 4.18: edit antrian diblokir tanpa langganan aktif.
	svc := newSpecQueueSvc(t)
	svc.On("UpdateQueue", int64(1), specOwner, "X", "").Return(apperror.ErrSubscriptionInactive)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPut, "/queues/1", `{"queue_name":"X"}`)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Equal(t, "SUBSCRIPTION_INACTIVE", specErrorCode(t, w))
}

func TestSpec_CloseQueue_OwnerGetsSuccess(t *testing.T) {
	// TC-02-04 / 4.1 langkah 5: owner menutup antrian.
	svc := newSpecQueueSvc(t)
	svc.On("CloseQueue", int64(3), specOwner).Return(nil)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPost, "/queues/3/close", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "close", data["action"])
	assert.Equal(t, float64(3), data["queue_id"])
	svc.verify()
}

func TestSpec_CloseQueue_NonOwnerIsNotFound(t *testing.T) {
	// TC-08-03: owner lain tidak bisa menutup antrian -> 404.
	svc := newSpecQueueSvc(t)
	svc.On("CloseQueue", int64(3), int64(99)).Return(apperror.ErrNotFound)
	e := specQueueEngine(svc, 99)

	w := specRequest(e, http.MethodPost, "/queues/3/close", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "NOT_FOUND", specErrorCode(t, w))
}

func TestSpec_QueueEndpoints_NonNumericIDIs400WithoutServiceCall(t *testing.T) {
	// Validasi input: id antrian harus angka; service tidak dipanggil.
	svc := newSpecQueueSvc(t)
	e := specQueueEngine(svc, specOwner)
	cases := []struct{ method, target, body string }{
		{http.MethodGet, "/queues/abc", ""},
		{http.MethodPut, "/queues/abc", `{"queue_name":"x"}`},
		{http.MethodPost, "/queues/abc/close", ""},
	}
	for _, c := range cases {
		w := specRequest(e, c.method, c.target, c.body)
		assert.Equal(t, http.StatusBadRequest, w.Code, c.method+" "+c.target)
		assert.Equal(t, "BAD_REQUEST", specErrorCode(t, w))
	}
	assert.Equal(t, 0, svc.count("ViewQueue"))
	assert.Equal(t, 0, svc.count("CloseQueue"))
}

func TestSpec_ListQueues_EmptyListIsJSONArray(t *testing.T) {
	// Kontrak API: daftar kosong berupa [] (bukan null).
	svc := newSpecQueueSvc(t)
	svc.On("ListQueue", specOwner).Return([]model.Queue{}, nil)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodGet, "/queues", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, []interface{}{}, data["queues"])
}

func TestSkipSettings_GetReturnsNullNWhenUnset(t *testing.T) {
	svc := newSpecQueueSvc(t)
	svc.On("GetSkipSettings", int64(1), specOwner).Return(model.SkipSettings{Fallback: "remove", MaxReturns: 1}, nil)
	w := specRequest(specQueueEngine(svc, specOwner), http.MethodGet, "/queues/1/settings/skip", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "remove", data["fallback"])
	assert.Nil(t, data["n"])
	assert.Equal(t, float64(1), data["max_returns"])
}

func TestSkipSettings_PutAfterN(t *testing.T) {
	svc := newSpecQueueSvc(t)
	in := model.SkipSettings{Fallback: "after_n", N: 2, MaxReturns: 3}
	svc.On("UpdateSkipSettings", int64(1), specOwner, in).Return(in, nil)
	w := specRequest(specQueueEngine(svc, specOwner), http.MethodPut, "/queues/1/settings/skip", `{"fallback":"after_n","n":2,"max_returns":3}`)

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["n"])
}

func TestSkipSettings_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{apperror.ErrInvalidSkipSettings, http.StatusBadRequest, "BAD_REQUEST"},
		{apperror.ErrSubscriptionInactive, http.StatusPaymentRequired, "SUBSCRIPTION_INACTIVE"},
		{apperror.ErrQueueClosed, http.StatusConflict, "QUEUE_CLOSED"},
		{apperror.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
	}
	for _, c := range cases {
		svc := newSpecQueueSvc(t)
		in := model.SkipSettings{Fallback: "back", MaxReturns: 1}
		svc.On("UpdateSkipSettings", int64(1), specOwner, in).Return(model.SkipSettings{}, c.err)
		w := specRequest(specQueueEngine(svc, specOwner), http.MethodPut, "/queues/1/settings/skip", `{"fallback":"back","max_returns":1}`)
		assert.Equal(t, c.status, w.Code)
		assert.Equal(t, c.code, specErrorCode(t, w))
	}
}

func TestSkipSettings_BadBodyIs400(t *testing.T) {
	w := specRequest(specQueueEngine(newSpecQueueSvc(t), specOwner), http.MethodPut, "/queues/1/settings/skip", `nope`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
