package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// Listen memanggil wake setiap NOTIFY mail_outbox, dan menyambung ulang bila koneksi putus. Hanya
// mempercepat: bila gagal, worker tetap jalan lewat polling. LISTEN memberi latensi rendah, polling memberi
// jaminan. LISTEN memakai koneksi pgx khusus (bukan dari pool sqlx) karena notifikasi hanya bisa dibaca lewat
// API pgx native (WaitForNotification), yang tidak ada di database/sql.
func Listen(
	ctx context.Context,
	dsn string,
	wake func(),
	log *slog.Logger,
) {
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Jalankan satu sesi LISTEN. Galat (dan konteks belum batal) dicatat, lalu disambung ulang.
		if err := listenOnce(
			ctx,
			dsn,
			wake,
		); err != nil && ctx.Err() == nil {
			// Catat sebagai peringatan saja: polling tetap mengirim email walau LISTEN putus.
			log.Warn("outbox listen lost, reconnecting (polling continues)", "err", err)
		}
		// Tunggu 3 detik sebelum menyambung ulang, kecuali shutdown dimulai.
		select {
		// Shutdown: berhenti menyambung ulang.
		case <-ctx.Done():
			return
		// Jeda mencegah loop sambung ulang yang rapat saat database mati.
		case <-time.After(3 * time.Second):
		}
	}
}

// listenOnce menjalankan satu sesi LISTEN sampai koneksi putus atau konteks batal.
func listenOnce(
	ctx context.Context,
	dsn string,
	wake func(),
) error {
	// Buka koneksi khusus untuk mendengarkan notifikasi.
	conn, err := pgx.Connect(ctx, dsn)
	// Database tidak terjangkau atau DSN tidak valid.
	if err != nil {
		return err
	}
	// Tutup koneksi saat sesi selesai (konteks bisa sudah batal, jadi pakai konteks baru).
	defer conn.Close(context.Background())
	// Daftarkan koneksi ke channel mail_outbox.
	if _, err := conn.Exec(ctx, "LISTEN mail_outbox"); err != nil {
		// Perintah LISTEN gagal: sesi diakhiri agar disambung ulang.
		return err
	}
	// Bangunkan worker sekali setelah tersambung, untuk baris yang menunggu selama koneksi putus.
	wake()
	// Tunggu notifikasi berulang kali.
	for {
		// Tunggu satu notifikasi. Galat (koneksi putus atau konteks batal) menghentikan sesi.
		if _, err := conn.WaitForNotification(ctx); err != nil {
			// Galat khusus yang tidak seharusnya terjadi di sini: lewati dan tunggu notifikasi berikutnya.
			if err == pgx.ErrNoRows {
				// Tunggu notifikasi berikutnya.
				continue
			}
			// Koneksi putus atau konteks batal: akhiri sesi.
			return err
		}
		// Ada notifikasi: bangunkan worker.
		wake()
	}
}
