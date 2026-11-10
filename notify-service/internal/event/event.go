package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Jenis event yang dikirim trigger database lewat kolom kind.
const (
	// KindCalled: nomor dipanggil ke counter. Penerima: pemilik nomor dan tiket waiting terdepan.
	KindCalled = "called"
	// KindRecalled: owner memanggil ulang nomor yang sedang dilayani. Penerima: pemilik nomor.
	KindRecalled = "recalled"
	// KindSkipped: nomor dilewati permanen. Penerima: pemilik nomor.
	KindSkipped = "skipped"
	// KindRequeued: nomor kembali menunggu (skip dengan fallback, atau counter dihapus).
	KindRequeued = "requeued"
)

// Alasan untuk KindRequeued, dikirim di Event.Reason.
const (
	// ReasonSkip: nomor kembali karena dilewati dengan fallback.
	ReasonSkip = "skip"
	// ReasonCounterClosed: nomor kembali karena counter yang melayaninya dihapus.
	ReasonCounterClosed = "counter_closed"
)

// Event adalah satu payload NOTIFY yang sudah diurai dari JSON.
type Event struct {
	// Kind: jenis event (called, recalled, skipped, requeued).
	Kind string `json:"kind"`
	// QueueID: id antrian tempat kejadian ini terjadi.
	QueueID int64 `json:"queue_id"`
	// TicketID: id tiket yang terkait event ini.
	TicketID int64 `json:"ticket_id"`
	// Reason dan Counter hanya terisi untuk requeued. Counter dibawa di payload karena counter yang
	// dihapus sudah tidak bisa dibaca dari database.
	// Reason: alasan nomor kembali menunggu (ReasonSkip atau ReasonCounterClosed). Kosong untuk jenis lain.
	Reason string `json:"reason,omitempty"`
	// Counter: nama counter yang dihapus, dibawa di payload. Kosong untuk jenis lain.
	Counter string `json:"counter,omitempty"`
}

// Parse mengurai payload JSON dari NOTIFY dan memeriksa bahwa kind, queue_id, dan ticket_id terisi.
func Parse(raw string) (Event, error) {
	// Siapkan Event kosong untuk diisi dari JSON.
	var e Event
	// JSON tidak valid: kembalikan galat decode.
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return Event{}, err
	}
	// Ada field wajib yang kosong (kind kosong, atau id bernilai nol): payload tidak lengkap, tolak.
	if e.Kind == "" || e.QueueID == 0 || e.TicketID == 0 {
		return Event{}, fmt.Errorf("incomplete event: %q", raw)
	}
	// Payload lengkap: kembalikan event.
	return e, nil
}

// Listen berjalan sampai koneksi putus atau ctx selesai. LISTEN hanya jalan di primary.
// onState dipanggil true setelah LISTEN aktif (untuk healthcheck); boleh nil.
func Listen(ctx context.Context, dsn string, handle func(Event), onState func(bool)) error {
	// Buka koneksi khusus untuk LISTEN (tidak dari pool), karena notifikasi hanya datang ke koneksi ini.
	conn, err := pgx.Connect(ctx, dsn)
	// Gagal membuka koneksi: kembalikan galat agar pemanggil menyambung ulang.
	if err != nil {
		return err
	}
	// Tutup koneksi saat fungsi selesai. Pakai context baru agar penutupan tetap jalan walau ctx sudah batal.
	defer conn.Close(context.Background())
	// Daftarkan koneksi ke channel queue_events. Gagal: kembalikan galat.
	if _, err := conn.Exec(ctx, "LISTEN queue_events"); err != nil {
		return err
	}
	// LISTEN aktif: catat di log.
	slog.Info("listening on queue_events")
	// Beri tahu pemanggil (healthcheck) bahwa LISTEN sudah aktif. onState boleh nil.
	if onState != nil {
		onState(true)
	}
	// Tunggu notifikasi berulang kali sampai koneksi putus atau ctx selesai.
	for {
		// Tunggu satu notifikasi. Galat (koneksi putus atau ctx batal) menghentikan loop.
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		// Urai payload notifikasi. Payload rusak dicatat lalu dilewati, tidak menghentikan loop.
		e, err := Parse(n.Payload)
		if err != nil {
			slog.Warn("skip bad event", "err", err)
			continue
		}
		// Serahkan event yang valid ke handler.
		handle(e)
	}
}
