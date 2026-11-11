package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"qms/notify-service/internal/model"

	"github.com/jackc/pgx/v5"
)

// Parse mengurai payload JSON dari NOTIFY dan memeriksa bahwa kind, queue_id, dan ticket_id terisi.
func Parse(
	raw string,
) (model.Event, error) {
	// Siapkan model.Event kosong untuk diisi dari JSON.
	var e model.Event
	// JSON tidak valid: kembalikan galat decode.
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return model.Event{}, err
	}
	// Ada field wajib yang kosong (kind kosong, atau id bernilai nol): payload tidak lengkap, tolak.
	if e.Kind == "" || e.QueueID == 0 || e.TicketID == 0 {
		return model.Event{}, fmt.Errorf("incomplete event: %q", raw)
	}
	// Payload lengkap: kembalikan event.
	return e, nil
}

// Listen berjalan sampai koneksi putus atau ctx selesai. LISTEN hanya jalan di primary.
// onState dipanggil true setelah LISTEN aktif (untuk healthcheck); boleh nil.
func Listen(
	ctx context.Context,
	dsn string,
	handle func(model.Event),
	onState func(bool),
) error {
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
		// Blok sampai ada notifikasi masuk di channel queue_events.
		n, err := conn.WaitForNotification(ctx)
		// Koneksi putus atau ctx batal: akhiri loop, pemanggil yang memutuskan sambung ulang.
		if err != nil {
			return err
		}
		// Urai payload notifikasi menjadi event.
		e, err := Parse(n.Payload)
		// Payload rusak dicatat lalu dilewati, supaya satu pesan buruk tidak menghentikan pendengar.
		if err != nil {
			slog.Warn("skip bad event", "err", err)
			continue
		}
		// Serahkan event yang valid ke handler.
		handle(e)
	}
}
