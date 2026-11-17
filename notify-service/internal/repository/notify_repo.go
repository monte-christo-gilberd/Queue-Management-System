package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"qms/notify-service/internal/model"

	// import kosong: mendaftarkan driver "pgx" ke database/sql, sehingga sqlx.Open("pgx", ...) dikenali
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// NotifyRepository: implementasi NotifyRepositoryInterface di atas PostgreSQL (sqlx dengan driver pgx). Memakai pool koneksi yang dibuat di main.
type NotifyRepository struct {
	// db: pool koneksi ke PostgreSQL.
	db *sqlx.DB
	// devices: true bila token FCM (push_devices) ikut dibaca. Hanya dinyalakan bila pengirim FCM aktif.
	devices bool
}

// NewNotifyRepository membuat NotifyRepository dari pool koneksi yang sudah dibuka. Token FCM tidak dibaca
// sampai EnableDevices dipanggil.
func NewNotifyRepository(db *sqlx.DB) *NotifyRepository {
	// Bungkus pool ke NotifyRepository.
	return &NotifyRepository{db: db}
}

// EnableDevices menyalakan pembacaan token FCM dari push_devices di Subscriptions. Dipanggil main bila
// kredensial FCM terpasang; tanpa itu hanya langganan Web Push yang dibaca.
func (s *NotifyRepository) EnableDevices() *NotifyRepository {
	s.devices = true
	return s
}

// Ticket mengambil data tiket, antrian, dan nama counter yang memanggilnya.
func (s *NotifyRepository) Ticket(
	ctx context.Context,
	ticketID int64,
) (model.Ticket, error) {

	const query = `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix,
		       CASE WHEN (SELECT count(*) FROM counters c0 WHERE c0.queue_id = t.queue_id) > 1 THEN cc.name END
		FROM tickets t
		JOIN queues q ON q.id = t.queue_id
		LEFT JOIN counters cc ON cc.id = t.called_counter_id
		WHERE t.id = $1
	`
	// Siapkan struct tujuan dan variabel untuk nama counter yang boleh null.
	var t model.Ticket
	var counter *string
	// Nama counter hanya diisi bila antrian punya lebih dari satu counter (D1).
	err := s.db.QueryRowContext(
		ctx,
		query,
		ticketID,
	).Scan(
		&t.ID,
		&t.QueueID,
		&t.Number,
		&t.UserID,
		&t.Status,
		&t.QueueName,
		&t.Prefix,
		&counter,
	)

	// Nama counter ada (tidak null): salin ke struct tiket.
	if counter != nil {
		t.CounterName = *counter
	}
	// Kembalikan tiket beserta galat dari query (mis. tiket tidak ada).
	return t, err
}

// PositionOf: posisi tiket di urutan tunggu (1 = terdepan). Dihitung dari sort_key, sama dengan backend.
func (s *NotifyRepository) PositionOf(
	ctx context.Context,
	ticketID int64,
) (int, error) {

	const query = `
		SELECT 1 + count(*) 
		FROM tickets o
		JOIN tickets t ON t.id = $1
		WHERE o.queue_id = t.queue_id 
		AND o.status = 'waiting' 
		AND o.sort_key < t.sort_key
	`
	// Siapkan variabel untuk menampung posisi.
	var pos int
	// Hitung tiket waiting yang sort_key-nya lebih kecil dari tiket ini, lalu tambah 1.
	err := s.db.QueryRowContext(
		ctx,
		query,
		ticketID,
	).Scan(&pos)

	// Kembalikan posisi beserta galat query.
	return pos, err
}

// FirstWaiting mengambil tiket waiting terdepan di antrian. Kembalikan nil (tanpa galat) bila tidak ada.
func (s *NotifyRepository) FirstWaiting(
	ctx context.Context,
	queueID int64,
) (*model.Ticket, error) {

	const query = `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix
		FROM tickets t JOIN queues q ON q.id = t.queue_id
		WHERE t.queue_id = $1 AND t.status = 'waiting'
		ORDER BY t.sort_key ASC LIMIT 1
	`

	// Siapkan struct tujuan untuk tiket.
	var t model.Ticket
	// Ambil satu tiket waiting dengan sort_key terkecil di antrian ini.
	err := s.db.QueryRowContext(
		ctx,
		query,
		queueID,
	).Scan(
		&t.ID,
		&t.QueueID,
		&t.Number,
		&t.UserID,
		&t.Status,
		&t.QueueName,
		&t.Prefix,
	)

	// Tidak ada tiket waiting: bukan galat, kembalikan nil.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	// Galat lain dari query: kembalikan galat.
	if err != nil {
		return nil, err
	}
	// Tiket ditemukan: kembalikan alamatnya.
	return &t, nil
}

// Subscriptions mengambil semua langganan push milik user: Web Push, lalu token FCM bila EnableDevices aktif.
// Galat membaca token FCM hanya dicatat, supaya Web Push tetap terkirim (mis. migrasi push_devices belum jalan).
func (s *NotifyRepository) Subscriptions(
	ctx context.Context,
	userID int64,
) ([]model.Subscription, error) {

	const query = `
		SELECT endpoint, p256dh, auth 
		FROM push_subscriptions 
		WHERE user_id = $1
	`
	// Jalankan query langganan milik user.
	rows, err := s.db.QueryContext(
		ctx,
		query,
		userID,
	)

	// Query gagal: kembalikan error.
	if err != nil {
		return nil, err
	}
	// Tutup hasil query setelah fungsi selesai, supaya koneksi kembali ke pool.
	defer rows.Close()
	// Kumpulkan langganan yang terbaca.
	var out []model.Subscription
	// Baca baris demi baris sampai habis.
	for rows.Next() {
		// Siapkan struct langganan untuk baris ini.
		var sub model.Subscription
		// Salin kolom ke struct. Galat pemindaian menghentikan pembacaan.
		if err := rows.Scan(
			&sub.Endpoint,
			&sub.P256dh,
			&sub.Auth,
		); err != nil {
			return nil, err
		}
		// Tandai sebagai Web Push, lalu tambahkan ke hasil.
		sub.Kind = model.KindWebPush
		out = append(out, sub)
	}
	// Galat saat membaca baris (rows.Err): kembalikan, sama seperti sebelumnya.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// FCM mati: cukup langganan Web Push.
	if !s.devices {
		return out, nil
	}
	// Tambahkan token FCM. Gagal di sini tidak membatalkan Web Push.
	devices, err := s.deviceTokens(ctx, userID)
	if err != nil {
		slog.Warn("read push_devices failed; sending Web Push only", "user_id", userID, "err", err)
		return out, nil
	}
	return append(out, devices...), nil
}

// deviceTokens mengambil semua token FCM milik user dari push_devices.
func (s *NotifyRepository) deviceTokens(
	ctx context.Context,
	userID int64,
) ([]model.Subscription, error) {

	const query = `
		SELECT token
		FROM push_devices
		WHERE user_id = $1
	`
	// Jalankan query token milik user.
	rows, err := s.db.QueryContext(ctx, query, userID)
	// Query gagal (mis. tabel belum dimigrasi, atau grant notify_ro belum ada): dikembalikan ke Subscriptions,
	// yang hanya mencatatnya supaya Web Push tetap terkirim.
	if err != nil {
		return nil, err
	}
	// Tutup hasil query setelah fungsi selesai.
	defer rows.Close()
	// Kumpulkan token sebagai langganan berjenis FCM.
	var out []model.Subscription
	for rows.Next() {
		// Tandai sebagai FCM supaya pusher.Dispatcher mengirim lewat FCM dan DeleteSubscription menghapus dari
		// push_devices.
		sub := model.Subscription{Kind: model.KindFCM}
		// Salin token ke struct. Galat pemindaian menghentikan pembacaan.
		if err := rows.Scan(&sub.Token); err != nil {
			return nil, err
		}
		// Tambahkan token ke hasil.
		out = append(out, sub)
	}
	// Kembalikan hasil beserta galat saat membaca baris (rows.Err).
	return out, rows.Err()
}

// DeleteSubscription menghapus langganan yang sudah tidak berlaku: Web Push menurut endpoint (push_subscriptions),
// FCM menurut token (push_devices). Satu-satunya penulisan di package ini.
func (s *NotifyRepository) DeleteSubscription(
	ctx context.Context,
	sub model.Subscription,
) error {

	const (
		webPushQuery = `
		DELETE FROM push_subscriptions 
		WHERE endpoint = $1
	`
		deviceQuery = `
		DELETE FROM push_devices
		WHERE token = $1
	`
	)
	// Pilih tabel dan kunci sesuai jenis langganan.
	query, key := webPushQuery, sub.Endpoint
	if sub.Channel() == model.KindFCM {
		query, key = deviceQuery, sub.Token
	}
	// Hapus baris langganan yang kuncinya cocok.
	_, err := s.db.ExecContext(
		ctx,
		query,
		key,
	)

	// Kembalikan galat eksekusi, bila ada.
	return err
}
