// Command sse-holder menahan puluhan ribu koneksi SSE ke backend dan meniru klien nyata untuk uji beban.
//
// Kenapa bukan k6: k6 memakai satu VU (goroutine ber-runtime JS) per koneksi, terlalu berat untuk puluhan ribu koneksi
// panjang. Program Go ini menahan satu goroutine ringan per koneksi (sekitar 10 sampai 20 KB), sehingga satu proses
// memegang puluhan ribu koneksi (plan k6, bagian 3).
//
// Yang dilakukan:
//  1. membaca tokens.json (hasil `make perf-seed`),
//  2. owner membaca token join tiap antrian lalu pelanggan 0..N-1 bergabung (hanya pemegang tiket boleh subscribe),
//  3. membuka N koneksi `GET /api/v1/queues/:id/event` dengan laju -ramp per detik,
//  4. owner menekan Next tiap -next-interval per antrian supaya event mengalir,
//  5. meniru klien: setelah event (kecuali queue_qr_updated), panggil `GET /status` setelah debounce 300 ms,
//  6. menahan selama -duration lalu mencetak ringkasan (dan menulis JSON bila -report diberikan).
//
// Latensi event = waktu diterima dikurangi `ts` di event (jam server). Akurat bila penahan dan server satu mesin atau
// jamnya disinkronkan; pada generator terdistribusi, pakai NTP atau chrony.
//
// Pemakaian:
//
//	sse-holder -base http://localhost:18080 -tokens perf/data/tokens.json -conns 10000 -duration 2m
//
// Sebelum menahan puluhan ribu koneksi: `ulimit -n` harus lebih besar dari jumlah koneksi, dan rentang port lokal
// (net.ipv4.ip_local_port_range) menentukan batas koneksi per pasangan IP sumber-tujuan (lihat perf/README.md).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// semua pengaturan lewat flag supaya Makefile mudah memanggilnya
	cfg := Config{}
	tokensPath := flag.String("tokens", "perf/data/tokens.json", "berkas tokens.json hasil make perf-seed")
	report := flag.String("report", "", "tulis ringkasan JSON ke berkas ini (opsional)")
	flag.StringVar(&cfg.BaseURL, "base", "http://localhost:18080", "alamat nginx uji")
	flag.IntVar(&cfg.Conns, "conns", 1000, "jumlah koneksi SSE yang ditahan")
	flag.IntVar(&cfg.Ramp, "ramp", 500, "koneksi baru per detik")
	flag.DurationVar(&cfg.Duration, "duration", time.Minute, "lama menahan koneksi setelah semuanya dibuka")
	flag.BoolVar(&cfg.Mimic, "mimic", true, "tiru klien: panggil GET /status setelah tiap event (debounce)")
	flag.DurationVar(&cfg.Debounce, "debounce", 300*time.Millisecond, "jeda penggabungan event sebelum memanggil status")
	flag.DurationVar(&cfg.NextInterval, "next-interval", 25*time.Second, "jeda Next per antrian oleh owner (0 = tanpa pemicu)")
	flag.IntVar(&cfg.PreloadWorkers, "preload-workers", 64, "goroutine yang membuat tiket sebelum SSE dibuka")
	allowPublic := flag.Bool("allow-public-target", false, "izinkan target di luar jaringan privat (jangan dipakai ke produksi)")
	flag.Parse()

	// pagar target: sama dengan assertSafeTarget di k6 (tolak host produksi dan non-privat)
	if err := assertSafeTarget(cfg.BaseURL, *allowPublic, os.Getenv("PERF_BLOCKED_HOSTS")); err != nil {
		fmt.Fprintln(os.Stderr, "sse-holder:", err)
		os.Exit(2)
	}
	data, err := loadTokens(*tokensPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sse-holder:", err)
		os.Exit(1)
	}

	// Ctrl+C atau SIGTERM menghentikan uji lebih awal tetapi tetap mencetak ringkasan
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := newHolder(cfg, data)
	fmt.Printf("sse-holder: %d koneksi ke %s (ramp %d/dtk, tahan %s, mimic=%v)\n", cfg.Conns, cfg.BaseURL, cfg.Ramp, cfg.Duration, cfg.Mimic)
	rep, err := h.run(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sse-holder:", err)
		os.Exit(1)
	}
	printReport(rep)
	if *report != "" {
		raw, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(*report, raw, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "sse-holder: tulis laporan:", err)
		}
	}
	// kode keluar 3: koneksi yang bertahan di bawah 99,5% (gerbang uji plan k6)
	if rep.HeldPercent < 99.5 {
		os.Exit(3)
	}
}

// printReport mencetak ringkasan yang mudah dibaca.
func printReport(r Report) {
	ms := func(p Percentiles) string {
		return fmt.Sprintf("n=%d p50=%.1fms p95=%.1fms p99=%.1fms maks=%.1fms", p.Count, p.P50, p.P95, p.P99, p.Max)
	}
	fmt.Println()
	fmt.Println("== Ringkasan sse-holder")
	fmt.Printf("   koneksi: target %d, puncak %d, bertahan di akhir %d (%.2f%%), gagal dibuka %d, putus di tengah %d\n",
		r.Target, r.PeakConnected, r.FinalConnected, r.HeldPercent, r.ConnectErrors, r.Dropped)
	fmt.Printf("   event diterima: %d  heartbeat: %d  Next dipicu: %d (galat %d)\n", r.Events, r.Heartbeats, r.NextCalls, r.NextErrors)
	fmt.Printf("   buka koneksi : %s\n", ms(r.ConnectLatency))
	fmt.Printf("   latensi event: %s\n", ms(r.EventLatency))
	fmt.Printf("   GET /status  : %s (galat %d dari %d)\n", ms(r.StatusLatency), r.StatusErrors, r.StatusCalls)
	gate := "LULUS"
	if r.HeldPercent < 99.5 {
		gate = "GAGAL"
	}
	fmt.Printf("   gerbang koneksi bertahan >= 99,5%%: %s\n", gate)
}
