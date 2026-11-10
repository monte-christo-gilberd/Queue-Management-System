package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

// CounterRepository: akses data counter per antrian (tabel counters), termasuk penguncian antrian dan
// nomor tiket yang sedang dilayani tiap counter.
type CounterRepository struct {
	db *sqlx.DB
}

// NewCounterRepository membuat CounterRepository yang membaca dan menulis lewat koneksi *sqlx.DB.
func NewCounterRepository(
	db *sqlx.DB,
) *CounterRepository {
	return &CounterRepository{
		db: db,
	}
}

// ListWithServing: semua counter antrian, counter default dulu, lalu urut dibuat. Tiap counter membawa nomor
// yang sedang dilayaninya (bila ada).
func (r *CounterRepository) ListWithServing(
	ctx context.Context,
	queueID int64,
) ([]model.CounterWithServing, error) {
	counters := []model.CounterWithServing{}
	const query = `
		SELECT
			c.id, c.queue_id, c.name, c.created_at, c.is_default,
			t.id AS serving_ticket_id,
			t.ticket_number AS serving_number
		FROM counters c
		LEFT JOIN tickets t ON t.called_counter_id = c.id AND t.status = 'currently_served'
		WHERE c.queue_id = $1
		ORDER BY c.is_default DESC, c.created_at ASC, c.id ASC
	`
	// Query ini mengambil semua counter antrian ini, dengan tiket yang sedang dilayani (jika ada) lewat LEFT JOIN, lalu mengisi slice counters.
	// Bila query gagal, error dikembalikan ke service dan hasil tidak dipakai.
	if err := r.db.SelectContext(ctx, &counters, query, queueID); err != nil {
		return nil, err
	}
	return counters, nil
}

// LockQueue mengunci baris antrian. Pembuatan counter diserialkan lewat kunci ini agar batas jumlah counter
// tidak terlampaui oleh dua permintaan bersamaan.
func (r *CounterRepository) LockQueue(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) error {
	var id int64
	// Ambil id antrian dengan FOR UPDATE di dalam transaksi tx; transaksi lain yang mengunci antrian yang sama harus menunggu sampai tx selesai.
	err := tx.GetContext(ctx, &id, `SELECT id FROM queues WHERE id = $1 FOR UPDATE`, queueID)
	// Bila query gagal (umumnya antrian tidak ada), kembalikan ErrNotFound.
	if err != nil {
		return apperror.ErrNotFound
	}
	return nil
}

// Names: nama semua counter antrian (untuk cek duplikat dan nama otomatis).
func (r *CounterRepository) Names(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) ([]string, error) {
	names := []string{}
	// Ambil nama semua counter di antrian ini ke slice names, dalam transaksi tx.
	err := tx.SelectContext(ctx, &names, `SELECT name FROM counters WHERE queue_id = $1`, queueID)
	return names, err
}

// Insert membuat counter baru. Nama yang sudah dipakai di antrian ini: ErrDuplicateCounterName.
func (r *CounterRepository) Insert(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	name string,
) (model.Counter, error) {
	var counter model.Counter
	const query = `
		INSERT INTO counters (queue_id, name, created_at, is_default)
		VALUES ($1, $2, now(), FALSE)
		RETURNING id, queue_id, name, created_at, is_default
	`
	// Query ini menyisipkan counter baru (bukan default) dengan waktu sekarang, lalu mengembalikan baris yang baru dibuat ke counter.
	err := tx.GetContext(ctx, &counter, query, queueID, name)
	// Bila error adalah pelanggaran UNIQUE (nama counter sudah ada di antrian ini), kembalikan ErrDuplicateCounterName.
	if isUniqueViolation(err) {
		return model.Counter{}, apperror.ErrDuplicateCounterName
	}
	// Error database lain dikembalikan apa adanya.
	if err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// Get mengunci satu counter milik antrian ini (FOR UPDATE). Counter antrian lain: ErrNotFound.
func (r *CounterRepository) Get(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) (model.Counter, error) {
	var counter model.Counter
	const query = `
		SELECT id, queue_id, name, created_at, is_default
		FROM counters
		WHERE id = $1 AND queue_id = $2
		FOR UPDATE
	`
	// Query ini mengambil satu counter yang id dan queue_id-nya cocok, sambil mengunci barisnya (FOR UPDATE), lalu mengisi counter.
	err := tx.GetContext(ctx, &counter, query, counterID, queueID)
	// Bila tidak ada baris yang cocok (counter tidak ada atau milik antrian lain), kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Counter{}, apperror.ErrNotFound
	}
	// Error database lain dikembalikan apa adanya.
	if err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// Rename mengganti nama counter. Nama ganda di antrian yang sama: ErrDuplicateCounterName.
func (r *CounterRepository) Rename(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
	name string,
) error {
	// Jalankan UPDATE untuk mengganti nama counter dengan syarat id dan queue_id cocok; result berisi info jumlah baris yang berubah.
	result, err := tx.ExecContext(
		ctx,
		`UPDATE counters SET name = $3 WHERE id = $1 AND queue_id = $2`,
		counterID,
		queueID,
		name,
	)
	// Bila nama bentrok dengan counter lain di antrian yang sama, kembalikan ErrDuplicateCounterName.
	if isUniqueViolation(err) {
		return apperror.ErrDuplicateCounterName
	}
	// Error database lain dikembalikan apa adanya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang benar-benar berubah oleh UPDATE tadi.
	n, err := result.RowsAffected()
	// Bila gagal membaca jumlah baris, kembalikan error-nya.
	if err != nil {
		return err
	}
	// Jika tidak ada baris yang berubah, counter tidak ditemukan di antrian ini.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// Delete menghapus counter. Nomor yang sedang dilayani counter ini harus sudah dilepas lebih dulu
// (TicketService.ReleaseCounterTicket), karena penghapusan hanya menyentuh baris counter.
func (r *CounterRepository) Delete(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) error {
	// Jalankan DELETE counter dengan syarat id dan queue_id cocok; result berisi info jumlah baris yang terhapus.
	result, err := tx.ExecContext(
		ctx,
		`DELETE FROM counters WHERE id = $1 AND queue_id = $2`,
		counterID,
		queueID,
	)
	// Error database dikembalikan apa adanya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terhapus oleh DELETE tadi.
	n, err := result.RowsAffected()
	// Bila gagal membaca jumlah baris, kembalikan error-nya.
	if err != nil {
		return err
	}
	// Jika tidak ada baris yang terhapus, counter tidak ditemukan di antrian ini.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// isUniqueViolation: galat pelanggaran UNIQUE dari Postgres (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	// Cari di dalam rantai error apakah ada *pgconn.PgError, lalu pastikan kodenya 23505 (pelanggaran UNIQUE).
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
