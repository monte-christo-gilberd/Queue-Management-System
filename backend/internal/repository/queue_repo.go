package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

// QueueRepository mengakses data antrian: tabel queues, queue_settings, dan counters.
type QueueRepository struct {
	// db: koneksi PostgreSQL untuk query di luar transaksi.
	db *sqlx.DB
}

// NewQueueRepository membuat QueueRepository yang memakai koneksi db.
func NewQueueRepository(
	db *sqlx.DB,
) *QueueRepository {
	return &QueueRepository{
		db: db,
	}
}

// CreateNewQueue membuat antrian baru beserta baris queue_settings dan counter default dalam tx yang diberikan service.
// Mengembalikan antrian yang baru dibuat, termasuk id-nya.
func (r *QueueRepository) CreateNewQueue(
	ctx context.Context,
	tx *sqlx.Tx,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, error) {

	var queue model.Queue
	const queueQuery = `
		INSERT INTO queues(
			owner_id,
			queue_name,
			queue_desc,
			status,
			created_at
		)
		VALUES (
			$1,
			$2,
			$3,
			'active',
			now()
		)
		RETURNING
			id,
			owner_id,
			queue_name,
			queue_desc,
			ticket_prefix,
			status,
			created_at
	`

	// Jalankan INSERT antrian baru dengan status 'active', lalu ambil kembali kolom antrian yang baru dibuat (RETURNING).
	err := tx.GetContext(
		ctx,
		&queue,
		queueQuery,
		ownerID,
		queueName,
		queueDescription,
	)
	// Jika INSERT antrian gagal, kembalikan error; transaksi dibatalkan oleh pemanggil.
	if err != nil {
		return model.Queue{}, err
	}

	// Setiap antrian baru wajib punya baris queue_settings (default: static, token null).
	// Sama tx dengan pembuatan antrian.
	// Jalankan INSERT queue_settings untuk antrian ini; ON CONFLICT DO NOTHING membuat aman bila baris sudah ada, lalu kembalikan error bila gagal.
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO queue_settings (queue_id) VALUES ($1) ON CONFLICT (queue_id) DO NOTHING`,
		queue.ID,
	); err != nil {
		return model.Queue{}, err
	}

	// Setiap antrian baru otomatis punya 1 counter default; nama counter
	// disembunyikan di UI karena nilainya otomatis.
	// Jalankan INSERT counter default "Counter 1" (is_default TRUE) untuk antrian ini, lalu kembalikan error bila gagal.
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO counters (queue_id, name, created_at, is_default) VALUES ($1, $2, now(), TRUE)`,
		queue.ID,
		"Counter 1",
	); err != nil {
		return model.Queue{}, err
	}

	return queue, nil
}

// GetQueueByID mengambil satu antrian berdasarkan queueID.
// Mengembalikan ErrNotFound bila antrian tidak ada.
func (r *QueueRepository) GetQueueByID(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {

	var queue model.Queue
	const queueQuery = `
		SELECT
			id,
			owner_id,
			queue_name,
			queue_desc,
			ticket_prefix,
			status,
			created_at,
			closed_at
		FROM queues
		WHERE id = $1
	`

	// Ambil satu baris antrian dengan id queueID dan isi hasilnya ke queue.
	err := r.db.GetContext(
		ctx,
		&queue,
		queueQuery,
		queueID,
	)
	// Jika tidak ada baris (ErrNoRows), kembalikan ErrNotFound agar service bisa membedakannya dari error database.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Queue{}, apperror.ErrNotFound
	}
	// Error lain dari database dikembalikan apa adanya.
	if err != nil {
		return model.Queue{}, err
	}

	return queue, nil
}

// ListQueueByOwner mengambil semua antrian milik ownerID, diurutkan dari yang terbaru.
// Bila activeOnly true, hanya antrian berstatus active yang dikembalikan.
func (r *QueueRepository) ListQueueByOwner(
	ctx context.Context,
	ownerID int64,
	activeOnly bool,
) ([]model.Queue, error) {

	var queues []model.Queue
	const query = `
		SELECT
			id,
			owner_id,
			queue_name,
			queue_desc,
			ticket_prefix,
			status,
			created_at,
			closed_at
		FROM queues
		WHERE owner_id = $1
		AND ($2 = false OR status = 'active')
		ORDER BY created_at DESC
	`
	// Jalankan query dan isi slice queues dengan semua baris yang cocok (SelectContext).
	err := r.db.SelectContext(
		ctx,
		&queues,
		query,
		ownerID,
		activeOnly,
	)
	// Jika query gagal, kembalikan slice kosong beserta error.
	if err != nil {
		return []model.Queue{}, err
	}

	return queues, nil
}

// SetTicketPrefix mengubah prefix nomor tiket antrian (sudah divalidasi service).
func (r *QueueRepository) SetTicketPrefix(
	ctx context.Context,
	queueID int64,
	prefix string,
) error {
	// Jalankan UPDATE ticket_prefix untuk antrian queueID; ExecContext tidak mengembalikan baris, hanya hasil eksekusi.
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE queues SET ticket_prefix = $1 WHERE id = $2`,
		prefix,
		queueID,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh oleh UPDATE tadi.
	n, err := result.RowsAffected()
	// Jika jumlah baris tidak bisa dibaca, kembalikan error.
	if err != nil {
		return err
	}
	// Jika tidak ada baris yang berubah, antrian dengan id itu tidak ada; kembalikan ErrNotFound.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// UpdateQueueDetails mengubah nama dan deskripsi antrian queueID.
// Nilai kosong tidak menimpa kolom yang ada. Mengembalikan ErrNotFound bila antrian tidak ada.
func (r *QueueRepository) UpdateQueueDetails(
	ctx context.Context,
	queueID int64,
	queueName,
	queueDescription string,
) error {

	const query = `
		UPDATE queues
		SET
			queue_name = COALESCE(NULLIF($1, ''), queue_name),
			queue_desc = COALESCE(NULLIF($2, ''), queue_desc)
		WHERE id = $3
	`

	// Jalankan UPDATE nama dan deskripsi antrian; COALESCE/NULLIF membuat nilai kosong berarti "tidak diubah".
	result, err := r.db.ExecContext(
		ctx,
		query,
		queueName,
		queueDescription,
		queueID,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh oleh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// Jika jumlah baris tidak bisa dibaca, kembalikan error.
	if err != nil {
		return err
	}

	// Tidak ada baris yang berubah berarti antrian tidak ditemukan.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}

// CloseQueue menutup antrian dan, dalam transaksi yang sama, menurunkan nomor yang masih sedang dilayani
// menjadi completed. Tanpa itu, nomor tersebut akan terus tampil "sedang dilayani" di antrian yang sudah tutup.
func (r *QueueRepository) CloseQueue(
	ctx context.Context,
	queueID int64,
) error {

	// Mulai transaksi baru agar penutupan antrian dan update tiket terjadi bersama.
	tx, err := r.db.BeginTxx(ctx, nil)
	// Jika gagal memulai transaksi, kembalikan error.
	if err != nil {
		return err
	}
	// Jika fungsi keluar sebelum Commit, transaksi di-rollback; setelah Commit berhasil, panggilan ini tidak berpengaruh.
	defer tx.Rollback()

	const closeQuery = `
		UPDATE queues
		SET status = 'closed', closed_at = now()
		WHERE
			id = $1 AND status = 'active'
	`
	// Jalankan UPDATE untuk menutup antrian queueID (hanya jika masih active) dan mencatat closed_at, dalam tx.
	result, err := tx.ExecContext(
		ctx,
		closeQuery,
		queueID,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh oleh UPDATE penutupan.
	rowsAffected, err := result.RowsAffected()
	// Jika jumlah baris tidak bisa dibaca, kembalikan error.
	if err != nil {
		return err
	}

	// Tidak ada baris yang berubah: antrian tidak ada atau sudah tutup; kembalikan ErrNotFound.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}

	const completeServingQuery = `
		UPDATE tickets
		SET status = 'completed', completed_at = now()
		WHERE queue_id = $1 AND status = 'currently_served'
	`
	// Jalankan UPDATE yang mengubah tiket currently_served di antrian ini menjadi completed; error dikembalikan bila gagal.
	if _, err := tx.ExecContext(
		ctx,
		completeServingQuery,
		queueID,
	); err != nil {
		return err
	}

	// Simpan semua perubahan dalam transaksi; bila Commit gagal, error dikembalikan.
	return tx.Commit()
}

// GetQueueSettings mengambil pengaturan QR (qr_type dan qr_current_code) untuk antrian queueID.
// Mengembalikan ErrNotFound bila baris queue_settings tidak ada.
func (r *QueueRepository) GetQueueSettings(
	ctx context.Context,
	queueID int64,
) (model.QueueSettings, error) {

	var settings model.QueueSettings
	const query = `
		SELECT
			queue_id,
			qr_type,
			qr_current_code,
			qr_expires_at
		FROM queue_settings
		WHERE queue_id = $1
	`

	// Ambil satu baris queue_settings untuk queueID dan isi hasilnya ke settings.
	err := r.db.GetContext(
		ctx,
		&settings,
		query,
		queueID,
	)
	// Jika query gagal, periksa dulu apakah error-nya karena baris tidak ada.
	if err != nil {
		// Jika baris queue_settings tidak ada, kembalikan ErrNotFound.
		if errors.Is(err, sql.ErrNoRows) {
			return model.QueueSettings{}, apperror.ErrNotFound
		}
		return model.QueueSettings{}, err
	}

	return settings, nil
}

// GetSkipSettings mengambil pengaturan skip antrian queueID.
// Bila baris queue_settings belum ada, pengaturan default dikembalikan tanpa error.
func (r *QueueRepository) GetSkipSettings(
	ctx context.Context,
	queueID int64,
) (model.SkipSettings, error) {
	var s model.SkipSettings
	// Ambil skip_fallback, skip_fallback_n (COALESCE ke 0 bila NULL), dan skip_max_returns untuk antrian queueID.
	err := r.db.GetContext(
		ctx,
		&s,
		`SELECT skip_fallback, COALESCE(skip_fallback_n, 0) AS skip_fallback_n, skip_max_returns
		 FROM queue_settings WHERE queue_id = $1`,
		queueID,
	)
	// Jika baris queue_settings belum ada, pakai pengaturan skip default tanpa error.
	if errors.Is(err, sql.ErrNoRows) {
		// Kembalikan pengaturan skip default dari package model.
		return model.DefaultSkipSettings(), nil
	}
	// Error lain dari database dikembalikan.
	if err != nil {
		return model.SkipSettings{}, err
	}
	return s, nil
}

// UpdateSkipSettings menyimpan pengaturan skip antrian queueID dengan upsert.
func (r *QueueRepository) UpdateSkipSettings(
	ctx context.Context,
	queueID int64,
	s model.SkipSettings,
) error {
	var n interface{}
	// Jika fallback berjenis "setelah N", simpan nilai N; selain itu n tetap nil sehingga kolom skip_fallback_n dikosongkan.
	if s.Fallback == model.SkipFallbackAfterN {
		n = s.N
	}
	// Baris queue_settings dibuat bersama antrian; upsert menjaga antrian lama tanpa baris.
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO queue_settings (queue_id, skip_fallback, skip_fallback_n, skip_max_returns)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (queue_id) DO UPDATE
		 SET skip_fallback = EXCLUDED.skip_fallback,
		     skip_fallback_n = EXCLUDED.skip_fallback_n,
		     skip_max_returns = EXCLUDED.skip_max_returns,
		     updated_at = now()`,
		queueID,
		s.Fallback,
		n,
		s.MaxReturns,
	)
	return err
}

// CreateQueueSettingsIfMissing menambahkan baris queue_settings untuk antrian queueID bila belum ada.
func (r *QueueRepository) CreateQueueSettingsIfMissing(
	ctx context.Context,
	queueID int64,
) error {
	// Jalankan INSERT queue_settings untuk antrian ini; ON CONFLICT DO NOTHING membuat aman bila baris sudah ada.
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO queue_settings (queue_id) VALUES ($1) ON CONFLICT (queue_id) DO NOTHING`,
		queueID,
	)
	// Jika INSERT gagal, kembalikan error.
	if err != nil {
		return err
	}
	// Baca jumlah baris terpengaruh; nilainya tidak dipakai, hanya untuk memastikan tidak ada error.
	_, err = result.RowsAffected()
	// Jika membaca jumlah baris gagal, kembalikan error.
	if err != nil {
		return err
	}
	return nil
}

// UpdateQRType mengubah qr_type di queue_settings untuk antrian queueID.
// Mengembalikan ErrNotFound bila tidak ada baris yang berubah.
func (r *QueueRepository) UpdateQRType(
	ctx context.Context,
	queueID int64,
	qrType string,
) error {
	// Jalankan UPDATE qr_type untuk antrian queueID.
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE queue_settings SET qr_type = $2 WHERE queue_id = $1`,
		queueID,
		qrType,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh oleh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// Jika jumlah baris tidak bisa dibaca, kembalikan error.
	if err != nil {
		return err
	}
	// Tidak ada baris yang berubah berarti queue_settings antrian itu tidak ada.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// UpdateQRCurrentCode mengubah qr_current_code di queue_settings untuk antrian queueID.
// Mengembalikan ErrNotFound bila tidak ada baris yang berubah.
func (r *QueueRepository) UpdateQRCurrentCode(
	ctx context.Context,
	queueID int64,
	token string,
) error {

	const query = `
		UPDATE queue_settings 
		SET 
			qr_current_code = $2, 
			qr_expires_at = NULL 
		WHERE queue_id = $1
	`

	// Jalankan UPDATE qr_current_code (token QR yang aktif) untuk antrian queueID.
	result, err := r.db.ExecContext(
		ctx,
		query,
		queueID,
		token,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh oleh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// Jika jumlah baris tidak bisa dibaca, kembalikan error.
	if err != nil {
		return err
	}
	// Tidak ada baris yang berubah berarti queue_settings antrian itu tidak ada.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// CreateDefaultCounter membuat counter baru (bukan default) dengan nama name untuk antrian queueID.
func (r *QueueRepository) CreateDefaultCounter(
	ctx context.Context,
	queueID int64,
	name string,
) (model.Counter, error) {
	var counter model.Counter
	const counterQuery = `
		INSERT INTO counters (queue_id, name, created_at, is_default)
		VALUES ($1, $2, now(), FALSE)
		RETURNING id, queue_id, name, created_at, is_default
	`
	// Jalankan INSERT counter baru dan ambil baris yang dibuat (RETURNING) ke counter; error dikembalikan bila gagal.
	if err := r.db.GetContext(ctx, &counter, counterQuery, queueID, name); err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// GetCounterByQueue mengambil satu counter untuk antrian queueID.
// Counter default diutamakan, lalu yang paling lama dibuat.
func (r *QueueRepository) GetCounterByQueue(
	ctx context.Context,
	queueID int64,
) (model.Counter, error) {
	var counter model.Counter
	const counterQuery = `
		SELECT id, queue_id, name, created_at, is_default
		FROM counters
		WHERE queue_id = $1
		ORDER BY is_default DESC, created_at ASC
		LIMIT 1
	`
	// Ambil satu counter teratas untuk antrian queueID; error dikembalikan bila gagal.
	if err := r.db.GetContext(ctx, &counter, counterQuery, queueID); err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// GetQueueSettingsByJoinCode mencari queue_settings dari kode join aktif. Mengembalikan ErrNotFound bila tidak ada.
func (r *QueueRepository) GetQueueSettingsByJoinCode(
	ctx context.Context,
	code string,
) (model.QueueSettings, error) {
	var settings model.QueueSettings
	// Cari baris yang kode aktifnya sama dengan code. Kode lama yang sudah diganti tidak ada di kolom ini.
	err := r.db.GetContext(
		ctx,
		&settings,
		`SELECT queue_id, qr_type, qr_current_code, qr_expires_at
		 FROM queue_settings WHERE qr_current_code = $1`,
		code,
	)
	// Tidak ada baris yang cocok: kode tidak dikenal, kembalikan ErrNotFound.
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.QueueSettings{}, apperror.ErrNotFound
		}
		// Error database lain dikembalikan apa adanya.
		return model.QueueSettings{}, err
	}
	// Baris ditemukan: kembalikan pengaturan antrian yang memakai kode ini.
	return settings, nil
}

// UpdateDynamicQRCode menyimpan kode dinamis baru beserta waktu kedaluwarsanya untuk antrian queueID.
func (r *QueueRepository) UpdateDynamicQRCode(
	ctx context.Context,
	queueID int64,
	code string,
	expiresAt time.Time,
) error {
	// Simpan kode dinamis dan masa berlakunya untuk antrian queueID.
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE queue_settings SET qr_current_code = $2, qr_expires_at = $3 WHERE queue_id = $1`,
		queueID,
		code,
		expiresAt,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh oleh UPDATE.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// Tidak ada baris yang berubah berarti queue_settings antrian itu tidak ada.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// RotateDynamicQRCode mengganti kode dinamis oldCode dengan newCode hanya bila kode aktif masih oldCode dan
// antrian memakai QR dinamis. Mengembalikan false bila kode sudah berganti (tidak ada baris yang berubah).
func (r *QueueRepository) RotateDynamicQRCode(
	ctx context.Context,
	queueID int64,
	oldCode string,
	newCode string,
	expiresAt time.Time,
) (bool, error) {
	// UPDATE bersyarat: kode hanya diganti bila masih oldCode dan mode antrian masih dinamis.
	// Bila join lain sudah lebih dulu menggantinya, tidak ada baris yang cocok dan hasilnya false.
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE queue_settings
		 SET qr_current_code = $3, qr_expires_at = $4
		 WHERE queue_id = $1 AND qr_current_code = $2 AND qr_type = 'dynamic'`,
		queueID,
		oldCode,
		newCode,
		expiresAt,
	)
	// Jika UPDATE gagal, kembalikan error.
	if err != nil {
		return false, err
	}
	// Ambil jumlah baris yang terpengaruh; lebih dari nol berarti kode benar-benar diganti.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}
