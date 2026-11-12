package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"qms/internal/middleware"
)

// newLogEngine membuat engine dengan urutan middleware yang sama seperti router produksi
// (RequestID, Logger, Recovery) dan buffer yang menampung log JSON-nya.
func newLogEngine(t *testing.T) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// log ditulis ke buffer; level Debug agar log probe /health (level debug) ikut tertangkap
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Logger(logger), middleware.Recovery(logger))
	return engine, buf
}

// logLines memecah buffer menjadi baris log yang sudah di-decode; gagal bila ada baris yang bukan JSON
// (semua keluaran log harus JSON agar bisa diurai collector).
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		// lewati baris kosong (buffer kosong menghasilkan satu elemen kosong)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("baris log bukan JSON: %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// Log akses memakai templat route dan tidak pernah memuat nilai parameter atau query string.
func TestLogger_UsesRouteTemplateAndHidesSecrets(t *testing.T) {
	engine, buf := newLogEngine(t)
	engine.GET("/display/:token", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// request dengan token di path dan di query, plus ID dari nginx
	req := httptest.NewRequest(http.MethodGet, "/display/SECRET-TOKEN-123?token=QUERY-SECRET", nil)
	req.Header.Set("X-Request-Id", "req-abc")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	// tidak boleh ada jejak rahasia di seluruh keluaran log
	if strings.Contains(buf.String(), "SECRET-TOKEN-123") || strings.Contains(buf.String(), "QUERY-SECRET") {
		t.Fatalf("log memuat nilai rahasia: %s", buf.String())
	}
	// harus tepat satu baris akses
	lines := logLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("diharapkan 1 baris log, dapat %d", len(lines))
	}
	got := lines[0]
	// atribut identitas request
	if got["route"] != "/display/:token" || got["method"] != "GET" || got["request_id"] != "req-abc" {
		t.Fatalf("atribut log tidak sesuai: %v", got)
	}
	// hasil: status, ukuran body ("ok" = 2 byte), dan level info
	if got["status"] != float64(200) || got["bytes"] != float64(2) || got["level"] != "INFO" {
		t.Fatalf("status/bytes/level tidak sesuai: %v", got)
	}
	// field wajib lain
	for _, key := range []string{"latency_ms", "client_ip"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("atribut %s tidak ada: %v", key, got)
		}
	}
	// request belum login, jadi tidak boleh ada user_id
	if _, ok := got["user_id"]; ok {
		t.Fatalf("user_id tidak boleh ada bila belum login: %v", got)
	}
}

// Rute yang tidak ada dicatat sebagai "unmatched", bukan path mentah, dengan level warn.
func TestLogger_UnmatchedRouteAndLevels(t *testing.T) {
	engine, buf := newLogEngine(t)

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tidak/ada/TOKEN-XYZ", nil))

	// path mentah tidak boleh bocor ke log
	if strings.Contains(buf.String(), "TOKEN-XYZ") {
		t.Fatalf("path mentah masuk log: %s", buf.String())
	}
	// 404 = kesalahan klien, jadi level WARN
	got := logLines(t, buf)[0]
	if got["route"] != "unmatched" || got["status"] != float64(404) || got["level"] != "WARN" {
		t.Fatalf("log 404 tidak sesuai: %v", got)
	}
}

// user_id dicatat sebagai angka bila middleware login sudah menyimpannya.
func TestLogger_RecordsNumericUserID(t *testing.T) {
	engine, buf := newLogEngine(t)
	engine.GET("/me", func(c *gin.Context) {
		// meniru middleware login yang menyimpan id user di context
		c.Set("user_id", int64(42))
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/me", nil))

	if got := logLines(t, buf)[0]; got["user_id"] != float64(42) {
		t.Fatalf("user_id seharusnya 42: %v", got)
	}
}

// Probe /health turun ke level debug supaya tidak memenuhi log (Docker memanggilnya terus-menerus).
func TestLogger_HealthProbeIsDebug(t *testing.T) {
	engine, buf := newLogEngine(t)
	engine.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if got := logLines(t, buf)[0]; got["level"] != "DEBUG" {
		t.Fatalf("level /health seharusnya DEBUG: %v", got)
	}
}

// Panic di handler menjadi 500 berbody error seragam; log panic dan log akses (status 500) sama-sama tercatat.
func TestRecovery_PanicBecomes500AndIsLogged(t *testing.T) {
	engine, buf := newLogEngine(t)
	engine.GET("/boom", func(c *gin.Context) { panic("meledak") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("X-Request-Id", "req-panic")
	engine.ServeHTTP(w, req)

	// klien menerima 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, diharapkan 500", w.Code)
	}
	// body berbentuk {"error":{"code":"INTERNAL_ERROR",...}} seperti galat handler biasa
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("body error tidak sesuai: %s", w.Body.String())
	}
	// detail panic tidak boleh sampai ke klien
	if strings.Contains(w.Body.String(), "meledak") {
		t.Fatalf("detail panic bocor ke klien: %s", w.Body.String())
	}

	// log: baris pertama panic (dengan request_id dan nilai panic), baris kedua akses berstatus 500
	lines := logLines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("diharapkan 2 baris log (panic + akses), dapat %d", len(lines))
	}
	if lines[0]["msg"] != "panic recovered" || lines[0]["request_id"] != "req-panic" || lines[0]["panic"] != "meledak" {
		t.Fatalf("log panic tidak sesuai: %v", lines[0])
	}
	if lines[1]["msg"] != "request" || lines[1]["status"] != float64(500) || lines[1]["level"] != "ERROR" {
		t.Fatalf("log akses panic tidak sesuai: %v", lines[1])
	}
}
