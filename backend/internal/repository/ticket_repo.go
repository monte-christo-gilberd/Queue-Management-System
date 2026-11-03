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

type TicketRepository struct {
	db *sqlx.DB
}

func NewTicketRepository(
	db *sqlx.DB,
) *TicketRepository {
	return &TicketRepository{
		db: db,
	}
}

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

	err := tx.GetContext(
		ctx,
		&max,
		query,
		queueID,
	)
	if err != nil {
		return 0, err
	}

	if !max.Valid {
		return 0, nil
	}
	return max.Int64, nil
}

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

	err := tx.GetContext(
		ctx,
		&newTicket,
		query,
		queueID,
		sortKey,
		userID,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.Ticket{}, apperror.ErrDuplicate
	}
	if err != nil {
		return model.Ticket{}, err
	}
	return newTicket, nil
}

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

	err := tx.GetContext(
		ctx,
		&currentTicket,
		query,
		queueID,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
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
	err := tx.GetContext(ctx, &t, query, queueID, counterID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
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
	result, err := tx.ExecContext(ctx, `UPDATE tickets SET recalled_at=now() WHERE id=$1`, ticketID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
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
	err := tx.GetContext(ctx, &s, query, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DefaultSkipSettings(), nil
	}
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
	result, err := tx.ExecContext(ctx, query, ticketID, sortKey)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
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
	if err := tx.GetContext(
		ctx,
		&base,
		`SELECT COALESCE(MAX(sort_key), 0) FROM tickets WHERE queue_id=$1`,
		queueID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE tickets SET sort_key = -sort_key WHERE queue_id=$1 AND status='waiting'`,
		queueID,
	); err != nil {
		return err
	}
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
	result, err := tx.ExecContext(ctx, query, ticketID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
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

	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
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

	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
		counterID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

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
	if err := r.db.GetContext(ctx, &counter, counterQuery, queueID); err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

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
	if len(statuses) == 0 {
		err := r.db.SelectContext(
			ctx,
			&tickets,
			r.db.Rebind(selectTickets+orderAndLimit),
			queueID,
		)
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
	if err != nil {
		return nil, err
	}

	err = r.db.SelectContext(
		ctx,
		&tickets,
		r.db.Rebind(query),
		args...,
	)
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

	err := r.db.GetContext(
		ctx,
		&exists,
		query,
		queueID,
		userID,
	)
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

	err := r.db.GetContext(
		ctx,
		&exists,
		query,
		queueID,
		userID,
	)
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

	err := r.db.GetContext(
		ctx,
		&ticket,
		query,
		queueID,
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
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
	err := r.db.GetContext(ctx, &counter, query, counterID, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Counter{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Counter{}, err
	}
	return counter, nil
}

// CurrentServedNumber: ticket_number tiket yang sedang dilayani; ok=false bila belum ada.
func (r *TicketRepository) CurrentServedNumber(
	ctx context.Context,
	queueID int64,
) (number int, ok bool, err error) {

	const query = `
		SELECT ticket_number
		FROM tickets
		WHERE queue_id = $1 AND status = 'currently_served'
		LIMIT 1
	`

	err = r.db.GetContext(
		ctx,
		&number,
		query,
		queueID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return number, true, nil
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

	err := r.db.GetContext(
		ctx,
		&count,
		query,
		queueID,
		sortKey,
	)
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

	err := r.db.SelectContext(
		ctx,
		&tickets,
		query,
		userID,
	)
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

	err := r.db.SelectContext(ctx, &numbers, query, queueID, n)
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

	err = r.db.GetContext(ctx, &number, query, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
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
	err := tx.GetContext(ctx, &t, query, ticketID, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
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
	err := tx.GetContext(ctx, &id, `SELECT id FROM counters WHERE id=$1 AND queue_id=$2 FOR SHARE`, counterID, queueID)
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
	err := tx.GetContext(ctx, &t, query, queueID, counterID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
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
	result, err := tx.ExecContext(ctx, query, ticketID, sortKey)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
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

	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

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

	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}
