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

// TicketRepository membungkus koneksi *sqlx.DB dan menyediakan query untuk tabel tickets.
type TicketRepository struct {
	// db: koneksi PostgreSQL untuk query di luar transaksi.
	db *sqlx.DB
}

// NewTicketRepository membuat TicketRepository yang memakai koneksi db untuk query tanpa transaksi.
func NewTicketRepository(
	db *sqlx.DB,
) *TicketRepository {
	return &TicketRepository{
		db: db,
	}
}

// MaxWaitingSortKey mengembalikan sort_key terbesar di antrian ini, atau 0 bila belum ada tiket.
// Dipakai untuk menentukan sort_key tiket baru.
func (r *TicketRepository) MaxWaitingSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (int64, error) {
	var max sql.NullInt64

	// Semua status, bukan hanya waiting: UNIQUE (queue_id, sort_key) berlaku untuk tiket yang sudah
	// completed juga. Kalau hanya waiting, antrian yang sudah habis memberi kunci yang bentrok.
	const query = `
		SELECT MAX(sort_key)
		FROM tickets
		WHERE queue_id=$1
	`

	// Ambil MAX(sort_key) antrian ini ke max. Antrian kosong menghasilkan NULL, karena itu dibaca lewat sql.NullInt64.
	err := tx.GetContext(
		ctx,
		&max,
		query,
		queueID,
	)
	// query gagal: kembalikan 0 dan error-nya.
	if err != nil {
		return 0, err
	}

	// MAX bernilai NULL (antrian belum punya tiket): kembalikan 0 tanpa error.
	if !max.Valid {
		return 0, nil
	}
	return max.Int64, nil
}

// Insert menambahkan tiket waiting baru di antrian ini dengan nomor berikutnya, lalu mengembalikan baris yang dibuat.
// Pelanggaran UNIQUE dikembalikan sebagai ErrDuplicate.
func (r *TicketRepository) Insert(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID, userID, sortKey int64,
) (model.Ticket, error) {

	var newTicket model.Ticket
	const query = `
		INSERT INTO tickets(
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status
		)
		VALUES (
			$1, (
			SELECT 
				COALESCE(MAX(ticket_number), 0) + 1 
			FROM tickets 
			WHERE queue_id = $1
			),
		 	$2, 
			$3, 
			'waiting'
		)
		RETURNING 
			id, 
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status, 
			skip_count, 
			last_notified_at, 
			created_at, 
			completed_at, 
			cancelled_at
	`

	// Jalankan INSERT tiket baru (status waiting, nomor = MAX+1 per antrian) dan baca kolom hasilnya lewat RETURNING ke newTicket.
	err := tx.GetContext(
		ctx,
		&newTicket,
		query,
		queueID,
		sortKey,
		userID,
	)

	var pgErr *pgconn.PgError
	// error dari PostgreSQL berkode 23505 (pelanggaran UNIQUE, mis. sort_key bentrok): kembalikan ErrDuplicate.
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.Ticket{}, apperror.ErrDuplicate
	}
	// error lain: kembalikan tiket kosong dan error asli.
	if err != nil {
		return model.Ticket{}, err
	}
	return newTicket, nil
}

// NextWaiting mengunci dan mengembalikan tiket waiting dengan sort_key terkecil di antrian ini.
// Tiket yang sedang dikunci transaksi lain dilewati. Tidak ada tiket: ErrNotFound.
func (r *TicketRepository) NextWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.Ticket, error) {

	var currentTicket model.Ticket
	const query = `
		SELECT  
			id, 
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status, 
			skip_count, 
			last_notified_at, 
			created_at, 
			completed_at, 
			cancelled_at
		FROM tickets 
		WHERE queue_id=$1 AND status='waiting'
		ORDER BY sort_key ASC 
		LIMIT 1 
		FOR UPDATE SKIP LOCKED
	`

	// Jalankan SELECT tiket waiting terkecil dengan FOR UPDATE SKIP LOCKED, hasilnya ke currentTicket.
	err := tx.GetContext(
		ctx,
		&currentTicket,
		query,
		queueID,
	)

	// tidak ada tiket waiting yang bisa dikunci: kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Ticket{}, err
	}

	return currentTicket, nil
}

// CurrentServing: nomor yang sedang dilayani counter ini, dikunci. Tidak ada: ErrNotFound.
func (r *TicketRepository) CurrentServing(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) (model.Ticket, error) {
	var t model.Ticket
	const query = `
		SELECT
			id, queue_id, ticket_number, sort_key, user_id, status, skip_count,
			last_notified_at, created_at, completed_at, cancelled_at, called_counter_id, called_at, recalled_at
		FROM tickets
		WHERE queue_id=$1 AND called_counter_id=$2 AND status='currently_served'
		FOR UPDATE
	`
	// Jalankan query untuk mengambil dan mengunci nomor yang sedang dilayani counter ini, hasilnya ke t.
	err := tx.GetContext(ctx, &t, query, queueID, counterID)
	// tidak ada baris: kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Ticket{}, err
	}
	return t, nil
}

// MarkRecalled menandai nomor yang sedang dilayani sudah dipanggil ulang. Trigger database memancarkan sinyal push.
func (r *TicketRepository) MarkRecalled(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {
	// Jalankan UPDATE recalled_at = sekarang untuk tiket ini. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(ctx, `UPDATE tickets SET recalled_at=now() WHERE id=$1`, ticketID)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang benar-benar terpengaruh UPDATE.
	n, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada baris yang berubah (tiket tidak ditemukan): kembalikan ErrNotFound.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// SkipSettings membaca pengaturan skip antrian di dalam transaksi yang sama dengan Skip.
func (r *TicketRepository) SkipSettings(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.SkipSettings, error) {
	var s model.SkipSettings
	const query = `
		SELECT skip_fallback, COALESCE(skip_fallback_n, 0) AS skip_fallback_n, skip_max_returns
		FROM queue_settings
		WHERE queue_id = $1
	`
	// Baca pengaturan skip antrian ini dari queue_settings ke s.
	err := tx.GetContext(ctx, &s, query, queueID)
	// antrian belum punya baris pengaturan: pakai nilai default.
	if errors.Is(err, sql.ErrNoRows) {
		// kembalikan pengaturan default dari package model, tanpa error.
		return model.DefaultSkipSettings(), nil
	}
	// error database lain: kembalikan nilai kosong dan error asli.
	if err != nil {
		return model.SkipSettings{}, err
	}
	return s, nil
}

// WaitingSortKeys: sort_key tiket waiting, terkecil dulu, paling banyak limit.
func (r *TicketRepository) WaitingSortKeys(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	limit int,
) ([]int64, error) {
	keys := []int64{}
	// Ambil daftar sort_key tiket waiting terkecil sebanyak limit ke keys.
	err := tx.SelectContext(
		ctx,
		&keys,
		`SELECT sort_key FROM tickets WHERE queue_id=$1 AND status='waiting' ORDER BY sort_key ASC LIMIT $2`,
		queueID,
		limit,
	)
	return keys, err
}

// SortKeyTaken: apakah sort_key sudah dipakai tiket mana pun di antrian ini (UNIQUE berlaku semua status).
func (r *TicketRepository) SortKeyTaken(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	sortKey int64,
) (bool, error) {
	var taken bool
	// Jalankan SELECT EXISTS untuk mengecek apakah sort_key ini sudah dipakai, hasilnya ke taken.
	err := tx.GetContext(
		ctx,
		&taken,
		`SELECT EXISTS (SELECT 1 FROM tickets WHERE queue_id=$1 AND sort_key=$2)`,
		queueID,
		sortKey,
	)
	return taken, err
}

// RequeueSkipped mengembalikan tiket yang dilewati ke waiting di sort_key baru. skip_count naik 1;
// penanda "sudah dipanggil" dikosongkan supaya tiket diperlakukan seperti tiket menunggu biasa.
func (r *TicketRepository) RequeueSkipped(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID,
	sortKey int64,
) error {
	const query = `
		UPDATE tickets
		SET status='waiting', skip_count=skip_count+1, sort_key=$2,
			completed_at=NULL, called_at=NULL, called_counter_id=NULL, recalled_at=NULL
		WHERE id=$1
	`
	// Jalankan UPDATE: kembalikan tiket ke waiting dengan sort_key baru dan naikkan skip_count. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(ctx, query, ticketID, sortKey)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh UPDATE.
	n, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada tiket dengan id ini: kembalikan ErrNotFound.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// RebalanceWaiting menomori ulang sort_key tiket waiting dengan jarak gap, urutan tetap. Kunci baru
// dimulai di atas sort_key terbesar mana pun supaya tidak bentrok dengan tiket non-waiting. Dua langkah
// (negasi lalu isi) karena UNIQUE (queue_id, sort_key) dicek per baris.
func (r *TicketRepository) RebalanceWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	gap int64,
) error {
	var base int64
	// Ambil sort_key terbesar antrian (0 bila kosong) ke base. Bila query gagal, kembalikan error-nya.
	if err := tx.GetContext(
		ctx,
		&base,
		`SELECT COALESCE(MAX(sort_key), 0) FROM tickets WHERE queue_id=$1`,
		queueID,
	); err != nil {
		return err
	}
	// Langkah 1: negasikan sort_key semua tiket waiting supaya nilai sementara tidak bentrok dengan nilai lama. Bila gagal, kembalikan error-nya.
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE tickets SET sort_key = -sort_key WHERE queue_id=$1 AND status='waiting'`,
		queueID,
	); err != nil {
		return err
	}
	// Langkah 2: isi sort_key baru = base + nomor urut * gap. Nomor urut mengikuti urutan lama (sort_key terkecil dapat nomor 1), sehingga urutan tetap.
	// Hasil query (jumlah baris) tidak dipakai, hanya error-nya yang dikembalikan.
	_, err := tx.ExecContext(
		ctx,
		`UPDATE tickets t SET sort_key = $2 + o.rn * $3
		 FROM (
			SELECT id, ROW_NUMBER() OVER (ORDER BY sort_key DESC) AS rn
			FROM tickets WHERE queue_id=$1 AND status='waiting'
		 ) o
		 WHERE t.id = o.id`,
		queueID,
		base,
		gap,
	)
	return err
}

// MarkSkipped mengubah tiket yang sedang dilayani menjadi skipped; called_at dipertahankan.
func (r *TicketRepository) MarkSkipped(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {
	const query = `
		UPDATE tickets
		SET status='skipped', skip_count=skip_count+1, completed_at=NULL
		WHERE id=$1
	`
	// Jalankan UPDATE: ubah tiket menjadi skipped dan naikkan skip_count. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(ctx, query, ticketID)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh UPDATE.
	n, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada tiket dengan id ini: kembalikan ErrNotFound.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// MarkCompleted menurunkan nomor yang sedang dilayani menjadi completed. called_at dan counter dipertahankan
// sebagai jejak pemanggilan; trigger database tidak memancarkan sinyal untuk perubahan ini.
func (r *TicketRepository) MarkCompleted(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {

	const query = `
		UPDATE tickets
		SET
			status='completed',
			completed_at=now()
		WHERE id=$1
	`

	// Jalankan UPDATE: ubah tiket menjadi completed dan isi completed_at dengan waktu sekarang.
	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)

	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada baris yang berubah (tiket tidak ditemukan): kembalikan ErrNotFound.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// MarkServing menaikkan tiket waiting menjadi nomor yang sedang dilayani: status, called_at, counter, dan
// penanda recall direset. Trigger database memancarkan sinyal 'called' dari perubahan ini.
func (r *TicketRepository) MarkServing(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
	counterID int64,
) error {

	const query = `
		UPDATE tickets
		SET
			status='currently_served',
			called_counter_id=$2,
			called_at=now(),
			recalled_at=NULL
		WHERE id=$1
	`

	// Jalankan UPDATE: ubah tiket waiting menjadi currently_served dengan counter dan waktu panggil. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
		counterID,
	)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada baris yang berubah (tiket tidak ditemukan): kembalikan ErrNotFound.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// GetCounterByQueue mengembalikan counter utama antrian: counter default lebih dulu, lalu yang paling lama dibuat.
// Tidak ada counter: error dari database dikembalikan apa adanya (tidak diterjemahkan ke ErrNotFound).
func (r *TicketRepository) GetCounterByQueue(
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
	// Ambil satu counter utama antrian ke counter. Bila query gagal, kembalikan error-nya.
	if err := r.db.GetContext(ctx, &counter, counterQuery, queueID); err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// ListByQueue mengembalikan tiket di antrian ini, diurutkan sort_key lalu id, maksimal 500.
// Bila statuses kosong semua status diambil; bila tidak, hanya status yang disebut.
func (r *TicketRepository) ListByQueue(
	ctx context.Context,
	queueID int64,
	statuses []string,
) ([]model.Ticket, error) {

	tickets := []model.Ticket{}

	// Catatan owner ikut lewat LEFT JOIN supaya daftar tiket bisa menampilkannya tanpa satu query per nomor.
	// Semua kolom diberi alias t. karena ticket_notes juga punya created_at.
	const selectTickets = `
		SELECT
			t.id,
			t.queue_id,
			t.ticket_number,
			t.sort_key,
			t.user_id,
			t.status,
			t.skip_count,
			t.last_notified_at,
			t.created_at,
			t.completed_at,
			t.cancelled_at,
			COALESCE(t.called_counter_id, 0) AS called_counter_id,
			t.called_at,
			n.note,
			n.updated_at AS note_updated_at
		FROM tickets t
		LEFT JOIN ticket_notes n ON n.ticket_id = t.id
		WHERE t.queue_id = ?
	`
	const orderAndLimit = `
		ORDER BY 
			t.sort_key ASC, 
			t.id ASC
		LIMIT 500
	`

	// Tanpa filter status: query statis biasa.
	// tidak ada filter status: pakai query statis biasa tanpa sqlx.In.
	if len(statuses) == 0 {
		// Jalankan SELECT tiket antrian ini tanpa filter status ke tickets. Rebind mengubah ? menjadi placeholder $1, $2, sesuai driver PostgreSQL.
		err := r.db.SelectContext(
			ctx,
			&tickets,
			r.db.Rebind(selectTickets+orderAndLimit),
			queueID,
		)
		// query gagal: kembalikan nil dan error-nya.
		if err != nil {
			return nil, err
		}
		return tickets, nil
	}

	// sqlx.In memperluas slice menjadi daftar placeholder; Rebind mengubah ? menjadi $1, $2, ...
	query, args, err := sqlx.In(
		selectTickets+` AND t.status IN (?)`+orderAndLimit,
		queueID,
		statuses,
	)
	// sqlx.In gagal (mis. slice tidak valid): kembalikan nil dan error-nya.
	if err != nil {
		return nil, err
	}

	// Jalankan SELECT tiket dengan filter status yang sudah di-Rebind, dengan args sebagai nilai parameternya, hasilnya ke tickets.
	err = r.db.SelectContext(
		ctx,
		&tickets,
		r.db.Rebind(query),
		args...,
	)
	// query gagal: kembalikan nil dan error-nya.
	if err != nil {
		return nil, err
	}

	return tickets, nil
}

// HasTicket melaporkan apakah user punya tiket aktif (waiting atau currently_served) di queue ini.
// Tiket yang sudah selesai tidak dihitung, supaya user boleh join lagi.
func (r *TicketRepository) HasTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {

	var exists bool

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM tickets
			WHERE queue_id = $1 AND user_id = $2
				AND status IN ('waiting', 'currently_served')
		)
	`

	// Jalankan SELECT EXISTS untuk mengecek apakah user punya tiket aktif di antrian ini, hasilnya ke exists.
	err := r.db.GetContext(
		ctx,
		&exists,
		query,
		queueID,
		userID,
	)
	// query gagal: kembalikan false dan error-nya.
	if err != nil {
		return false, err
	}

	return exists, nil
}

// HasAnyTicket melaporkan apakah user pernah punya tiket di queue ini, dalam status apa pun. Dipakai untuk
// hak baca queue (ViewQueue), supaya pemegang tiket yang sudah selesai tetap bisa melihat antrian.
func (r *TicketRepository) HasAnyTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {

	var exists bool

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM tickets
			WHERE queue_id = $1 AND user_id = $2
		)
	`

	// Jalankan SELECT EXISTS untuk mengecek apakah user pernah punya tiket di antrian ini (status apa pun), hasilnya ke exists.
	err := r.db.GetContext(
		ctx,
		&exists,
		query,
		queueID,
		userID,
	)
	// query gagal: kembalikan false dan error-nya.
	if err != nil {
		return false, err
	}

	return exists, nil
}

// FindByUser mengembalikan tiket user di queue ini. Tiket aktif (currently_served, lalu waiting) didahulukan;
// bila tidak ada, tiket terakhir yang sudah selesai.
func (r *TicketRepository) FindByUser(
	ctx context.Context,
	queueID,
	userID int64,
) (model.Ticket, error) {

	var ticket model.Ticket

	const query = `
		SELECT
			id,
			queue_id,
			ticket_number,
			sort_key,
			user_id,
			status,
			skip_count,
			last_notified_at,
			created_at,
			completed_at,
			cancelled_at,
			COALESCE(called_counter_id, 0) AS called_counter_id,
			called_at
		FROM tickets
		WHERE queue_id = $1 AND user_id = $2
		ORDER BY
			CASE status
				WHEN 'currently_served' THEN 0
				WHEN 'waiting' THEN 1
				ELSE 2
			END,
			id DESC
		LIMIT 1
	`

	// Jalankan query tiket user (prioritas aktif, lalu tiket terakhir) dan isi ticket dengan satu baris teratas.
	err := r.db.GetContext(
		ctx,
		&ticket,
		query,
		queueID,
		userID,
	)
	// user tidak punya tiket sama sekali di antrian ini: kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Ticket{}, err
	}

	return ticket, nil
}

// GetCounter mengembalikan counter milik antrian ini. Counter antrian lain atau id tidak dikenal: ErrNotFound.
func (r *TicketRepository) GetCounter(
	ctx context.Context,
	queueID,
	counterID int64,
) (model.Counter, error) {
	var counter model.Counter
	const query = `
		SELECT id, queue_id, name, created_at, is_default
		FROM counters
		WHERE id = $1 AND queue_id = $2
	`
	// Jalankan SELECT counter berdasarkan id dan queue_id, hasilnya ke counter.
	err := r.db.GetContext(ctx, &counter, query, counterID, queueID)
	// tidak ada baris (id tidak dikenal atau milik antrian lain): kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Counter{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// CountWaitingBefore: jumlah tiket waiting di queue ini dengan sort_key lebih kecil.
func (r *TicketRepository) CountWaitingBefore(
	ctx context.Context,
	queueID,
	sortKey int64,
) (int, error) {

	var count int

	const query = `
		SELECT COUNT(*)
		FROM tickets
		WHERE queue_id = $1
			AND status = 'waiting'
			AND sort_key < $2
	`

	// Jalankan SELECT COUNT tiket waiting dengan sort_key lebih kecil dari sortKey, hasilnya ke count.
	err := r.db.GetContext(
		ctx,
		&count,
		query,
		queueID,
		sortKey,
	)
	// query gagal: kembalikan 0 dan error-nya.
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ListByUser mengembalikan satu baris per antrian untuk user, terbaru dulu (maks 100). Per antrian dipilih
// tiket aktif bila ada (currently_served, lalu waiting), selain itu tiket terakhir. Pemilihan dilakukan di
// subquery karena DISTINCT ON mewajibkan ORDER BY diawali queue_id, sedangkan urutan tampilan memakai created_at.
func (r *TicketRepository) ListByUser(
	ctx context.Context,
	userID int64,
) ([]model.UserTicket, error) {

	tickets := []model.UserTicket{}

	const query = `
		SELECT * FROM (
			SELECT DISTINCT ON (t.queue_id)
				t.id,
				t.queue_id,
				t.ticket_number,
				t.sort_key,
				t.user_id,
				t.status,
				t.skip_count,
				t.last_notified_at,
				t.created_at,
				t.completed_at,
				t.cancelled_at,
				COALESCE(t.called_counter_id, 0) AS called_counter_id,
				t.called_at,
				q.queue_name,
				q.ticket_prefix,
				q.status AS queue_status,
				(
					SELECT c.ticket_number
					FROM tickets c
					WHERE c.queue_id = t.queue_id AND c.status = 'currently_served'
					LIMIT 1
				) AS current_number
			FROM tickets t
			JOIN queues q ON q.id = t.queue_id
			WHERE t.user_id = $1
			ORDER BY
				t.queue_id,
				CASE t.status
					WHEN 'currently_served' THEN 0
					WHEN 'waiting' THEN 1
					ELSE 2
				END,
				t.id DESC
		) r
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT 100
	`

	// Jalankan query daftar tiket terbaru per antrian milik user, hasilnya ke tickets.
	err := r.db.SelectContext(
		ctx,
		&tickets,
		query,
		userID,
	)
	// query gagal: kembalikan nil dan error-nya.
	if err != nil {
		return nil, err
	}

	return tickets, nil
}

// RecentCompletedNumbers: ticket_number tiket completed terbaru, maksimal n, terbaru dulu.
func (r *TicketRepository) RecentCompletedNumbers(
	ctx context.Context,
	queueID int64,
	n int,
) ([]int, error) {

	numbers := []int{}

	const query = `
		SELECT ticket_number
		FROM tickets
		WHERE queue_id = $1 AND status = 'completed'
		ORDER BY completed_at DESC, id DESC
		LIMIT $2
	`

	// Jalankan SELECT n nomor tiket completed terbaru di antrian ini, hasilnya ke numbers.
	err := r.db.SelectContext(ctx, &numbers, query, queueID, n)
	// query gagal: kembalikan nil dan error-nya.
	if err != nil {
		return nil, err
	}
	return numbers, nil
}

// FirstWaitingNumber: ticket_number tiket waiting dengan sort_key terkecil; ok=false bila tidak ada.
func (r *TicketRepository) FirstWaitingNumber(
	ctx context.Context,
	queueID int64,
) (number int, ok bool, err error) {

	const query = `
		SELECT ticket_number
		FROM tickets
		WHERE queue_id = $1 AND status = 'waiting'
		ORDER BY sort_key ASC, id ASC
		LIMIT 1
	`

	// Jalankan query nomor tiket waiting terdepan, hasilnya ke number.
	err = r.db.GetContext(ctx, &number, query, queueID)
	// tidak ada tiket waiting: kembalikan 0 dengan ok=false tanpa error.
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return 0, false, err
	}
	return number, true, nil
}

// GetForUpdate mengunci satu tiket di antrian ini (FOR UPDATE). Tiket milik antrian lain: ErrNotFound.
func (r *TicketRepository) GetForUpdate(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	ticketID int64,
) (model.Ticket, error) {
	var t model.Ticket
	const query = `
		SELECT
			id, queue_id, ticket_number, sort_key, user_id, status, skip_count,
			last_notified_at, created_at, completed_at, cancelled_at, COALESCE(called_counter_id, 0) AS called_counter_id, called_at, recalled_at
		FROM tickets
		WHERE id = $1 AND queue_id = $2
		FOR UPDATE
	`
	// Jalankan SELECT ... FOR UPDATE untuk mengunci tiket ini, hasilnya ke t.
	err := tx.GetContext(ctx, &t, query, ticketID, queueID)
	// tidak ada baris (tiket tidak ada atau milik antrian lain): kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Ticket{}, err
	}
	return t, nil
}

// LockCounter mengunci baris counter (FOR SHARE) supaya tidak dihapus di tengah Next/Skip/Recall. Bukan counter
// antrian ini: ErrNotFound.
func (r *TicketRepository) LockCounter(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) error {
	var id int64
	// Jalankan SELECT ... FOR SHARE untuk mengunci baris counter ini. id terisi hanya bila counter ada di antrian ini.
	err := tx.GetContext(ctx, &id, `SELECT id FROM counters WHERE id=$1 AND queue_id=$2 FOR SHARE`, counterID, queueID)
	// counter tidak ditemukan di antrian ini: kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrNotFound
	}
	return err
}

// ServingByQueue: semua nomor yang sedang dilayani di antrian, yang paling baru dipanggil dulu.
func (r *TicketRepository) ServingByQueue(
	ctx context.Context,
	queueID int64,
) ([]model.Ticket, error) {
	tickets := []model.Ticket{}
	const query = `
		SELECT
			id, queue_id, ticket_number, sort_key, user_id, status, skip_count,
			last_notified_at, created_at, completed_at, cancelled_at, called_counter_id, called_at, recalled_at
		FROM tickets
		WHERE queue_id = $1 AND status = 'currently_served'
		ORDER BY called_at DESC, id DESC
	`
	// Ambil semua tiket currently_served antrian ini ke tickets. Bila query gagal, kembalikan error-nya.
	if err := r.db.SelectContext(ctx, &tickets, query, queueID); err != nil {
		return nil, err
	}
	return tickets, nil
}

// ServingOfCounter mengunci nomor yang sedang dilayani counter ini (FOR UPDATE). Tidak ada: ErrNotFound.
func (r *TicketRepository) ServingOfCounter(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	counterID int64,
) (model.Ticket, error) {
	var t model.Ticket
	const query = `
		SELECT
			id, queue_id, ticket_number, sort_key, user_id, status, skip_count,
			last_notified_at, created_at, completed_at, cancelled_at, called_counter_id, called_at, recalled_at
		FROM tickets
		WHERE queue_id = $1 AND called_counter_id = $2 AND status = 'currently_served'
		FOR UPDATE
	`
	// Jalankan query nomor yang sedang dilayani counter ini dengan FOR UPDATE, hasilnya ke t.
	err := tx.GetContext(ctx, &t, query, queueID, counterID)
	// tidak ada baris: kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	// error database lain: kembalikan error asli.
	if err != nil {
		return model.Ticket{}, err
	}
	return t, nil
}

// ReturnToWaiting mengembalikan nomor yang sedang dilayani ke urutan tunggu dengan sort_key baru. Tidak menaikkan
// skip_count (bukan skip); penanda panggilan dikosongkan. Trigger tidak memancarkan sinyal untuk perubahan ini.
func (r *TicketRepository) ReturnToWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID,
	sortKey int64,
) error {
	const query = `
		UPDATE tickets
		SET status = 'waiting', sort_key = $2,
			called_counter_id = NULL, called_at = NULL, completed_at = NULL, recalled_at = NULL
		WHERE id = $1 AND status = 'currently_served'
	`
	// Jalankan UPDATE: kembalikan nomor yang dilayani ke waiting dengan sort_key baru dan kosongkan penanda panggilan. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(ctx, query, ticketID, sortKey)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}
	// Ambil jumlah baris yang terpengaruh UPDATE.
	n, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}
	// tidak ada baris yang berubah (tiket bukan currently_served): kembalikan ErrNotFound.
	if n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// MarkDeleted menghapus nomor dari antrian atas perintah owner (4.10). Perilakunya seperti skip tanpa fallback:
// status deleted, tidak dikembalikan ke waiting, dan skip_count tidak naik. called_at dipertahankan.
func (r *TicketRepository) MarkDeleted(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {

	const query = `
		UPDATE tickets
		SET
			status = 'deleted',
			completed_at = NULL
		WHERE id = $1
	`

	// Jalankan UPDATE: ubah tiket menjadi deleted dan kosongkan completed_at. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)
	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}

	// tidak ada baris yang berubah (tiket tidak ditemukan): kembalikan ErrNotFound.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}

// MarkCancelled membatalkan tiket waiting atau currently_served. called_at dan counter dipertahankan.
// Berjalan dalam transaksi pemanggil, supaya pembatalan nomor yang sedang dilayani dan promosi
// nomor berikutnya selalu terjadi bersamaan.
func (r *TicketRepository) MarkCancelled(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {

	const query = `
		UPDATE tickets
		SET
			status = 'cancelled',
			cancelled_at = now(),
			completed_at = NULL
		WHERE id = $1
	`

	// Jalankan UPDATE: ubah tiket menjadi cancelled dan isi cancelled_at dengan waktu sekarang. result berisi jumlah baris yang terpengaruh.
	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)

	// UPDATE gagal: kembalikan error-nya.
	if err != nil {
		return err
	}

	// Ambil jumlah baris yang terpengaruh UPDATE.
	rowsAffected, err := result.RowsAffected()
	// gagal membaca jumlah baris: kembalikan error-nya.
	if err != nil {
		return err
	}

	// tidak ada baris yang berubah (tiket tidak ditemukan): kembalikan ErrNotFound.
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}
