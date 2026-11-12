package metrics_test

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"qms/qr-service/internal/metrics"
)

// freeAddr meminta port kosong dari OS lalu melepasnya, supaya tes tidak bentrok dengan port 9100 milik stack asli.
func freeAddr(t *testing.T) string {
	t.Helper()
	// port 0 = biarkan OS memilih port yang bebas
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// lepas port segera; alamatnya sudah kita simpan
	defer l.Close()
	return l.Addr().String()
}

// Listener metrik menyajikan /metrics berisi metrik runtime Go dan label service.
func TestServe_ServesMetrics(t *testing.T) {
	// arahkan listener ke port bebas lewat METRICS_ADDR
	addr := freeAddr(t)
	t.Setenv("METRICS_ADDR", addr)
	m := metrics.New("test-svc")
	// t.Context() dibatalkan otomatis di akhir tes, sehingga listener ikut berhenti
	m.Serve(t.Context())

	// listener jalan di goroutine, jadi coba beberapa kali sampai siap (maks sekitar 2,5 detik)
	var body string
	for range 50 {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// harus memuat metrik runtime Go dan label service
	if !strings.Contains(body, "go_goroutines") || !strings.Contains(body, `service="test-svc"`) {
		t.Fatalf("keluaran /metrics tidak sesuai:\n%.300s", body)
	}
	// hanya /metrics yang ada di listener ini: rute lain harus 404 (listener bukan API)
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
	// ctx tes: dibatalkan otomatis di akhir tes
	ctx := t.Context()

	// "off": Serve harus kembali segera tanpa membuka port
	t.Setenv("METRICS_ADDR", "off")
	metrics.New("a").Serve(ctx)

	// port yang sudah dipakai proses lain: Serve hanya boleh mencatat peringatan
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	t.Setenv("METRICS_ADDR", busy.Addr().String())
	metrics.New("b").Serve(ctx)
	// beri waktu goroutine mencoba listen dan mencatat peringatan; tes lulus bila tidak ada panic
	time.Sleep(100 * time.Millisecond)
}
