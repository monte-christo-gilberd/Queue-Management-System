package repository

import (
	"context"
	"errors"

	"qms/notify-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotifyRepository: implementasi NotifyRepositoryInterface di atas PostgreSQL. Memakai pool koneksi yang dibuat di main.
type NotifyRepository struct {
	// pool: pool koneksi ke PostgreSQL.
	pool *pgxpool.Pool
}

// NewNotifyRepository membuat NotifyRepository dari pool koneksi yang sudah dibuka.
func NewNotifyRepository(pool *pgxpool.Pool) *NotifyRepository {
	// Bungkus pool ke NotifyRepository.
	return &NotifyRepository{pool: pool}
}

// Ticket mengambil data tiket, antrian, dan nama counter yang memanggilnya.
func (s *NotifyRepository) Ticket(ctx context.Context, ticketID int64) (model.Ticket, error) {
	// Siapkan struct tujuan dan variabel untuk nama counter yang boleh null.
	var t model.Ticket
	var counter *string
	// Nama counter hanya diisi bila antrian punya lebih dari satu counter (D1).
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix,
		       CASE WHEN (SELECT count(*) FROM counters c0 WHERE c0.queue_id = t.queue_id) > 1 THEN cc.name END
		FROM tickets t
		JOIN queues q ON q.id = t.queue_id
		LEFT JOIN counters cc ON cc.id = t.called_counter_id
		WHERE t.id = $1`, ticketID).Scan(&t.ID, &t.QueueID, &t.Number, &t.UserID, &t.Status, &t.QueueName, &t.Prefix, &counter)
	// Nama counter ada (tidak null): salin ke struct tiket.
	if counter != nil {
		t.CounterName = *counter
	}
	// Kembalikan tiket beserta galat dari query (mis. tiket tidak ada).
	return t, err
}

// PositionOf: posisi tiket di urutan tunggu (1 = terdepan). Dihitung dari sort_key, sama dengan backend.
func (s *NotifyRepository) PositionOf(ctx context.Context, ticketID int64) (int, error) {
	// Siapkan variabel untuk menampung posisi.
	var pos int
	// Hitung tiket waiting yang sort_key-nya lebih kecil dari tiket ini, lalu tambah 1.
	err := s.pool.QueryRow(ctx, `
		SELECT 1 + count(*) FROM tickets o
		JOIN tickets t ON t.id = $1
		WHERE o.queue_id = t.queue_id AND o.status = 'waiting' AND o.sort_key < t.sort_key`, ticketID).Scan(&pos)
	// Kembalikan posisi beserta galat query.
	return pos, err
}

// FirstWaiting mengambil tiket waiting terdepan di antrian. Kembalikan nil (tanpa galat) bila tidak ada.
func (s *NotifyRepository) FirstWaiting(ctx context.Context, queueID int64) (*model.Ticket, error) {
	// Siapkan struct tujuan untuk tiket.
	var t model.Ticket
	// Ambil satu tiket waiting dengan sort_key terkecil di antrian ini.
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix
		FROM tickets t JOIN queues q ON q.id = t.queue_id
		WHERE t.queue_id = $1 AND t.status = 'waiting'
		ORDER BY t.sort_key ASC LIMIT 1`, queueID).Scan(&t.ID, &t.QueueID, &t.Number, &t.UserID, &t.Status, &t.QueueName, &t.Prefix)
	// Tidak ada tiket waiting: bukan galat, kembalikan nil.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	// Galat lain dari query: kembalikan galat.
	if err != nil {
		return nil, err
	}
	// Tiket ditemukan: kembalikan alamatnya.
	return &t, nil
}

// Subscriptions mengambil semua langganan push milik user.
func (s *NotifyRepository) Subscriptions(ctx context.Context, userID int64) ([]model.Subscription, error) {
	// Jalankan query langganan milik user.
	rows, err := s.pool.Query(ctx, `SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	// Query gagal: kembalikan galat.
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
		if err := rows.Scan(&sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return nil, err
		}
		// Tambahkan langganan ke hasil.
		out = append(out, sub)
	}
	// Kembalikan hasil beserta galat yang terjadi saat membaca baris (rows.Err).
	return out, rows.Err()
}

// DeleteSubscription menghapus langganan dengan endpoint tertentu. Satu-satunya penulisan di package ini.
func (s *NotifyRepository) DeleteSubscription(ctx context.Context, endpoint string) error {
	// Hapus baris langganan yang endpoint-nya cocok.
	_, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	// Kembalikan galat eksekusi, bila ada.
	return err
}
