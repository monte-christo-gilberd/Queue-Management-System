package metrics_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"qms/internal/metrics"
)

// freeAddr meminta port kosong dari OS lalu melepasnya, supaya tes tidak bentrok dengan port 9100.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// Listener metrik menyajikan /metrics berisi metrik runtime Go dan label service.
func TestServe_ServesMetrics(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv("METRICS_ADDR", addr)
	m := metrics.New("test-svc")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Serve(ctx)

	var body string
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(body, "go_goroutines") || !strings.Contains(body, `service="test-svc"`) {
		t.Fatalf("keluaran /metrics tidak sesuai:\n%.300s", body)
	}
	// hanya /metrics yang ada di listener ini
	resp, err := http.Get("http://" + addr + "/api/v1/anything")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("rute selain /metrics seharusnya 404, dapat %d", resp.StatusCode)
	}
}

// METRICS_ADDR=off mematikan listener, dan port yang sudah terpakai tidak membuat Serve panik atau memblokir.
func TestServe_OffAndBusyPortDoNotFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Setenv("METRICS_ADDR", "off")
	metrics.New("a").Serve(ctx)

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	t.Setenv("METRICS_ADDR", busy.Addr().String())
	metrics.New("b").Serve(ctx)
	time.Sleep(100 * time.Millisecond) // beri waktu goroutine mencoba listen dan mencatat peringatan
}
