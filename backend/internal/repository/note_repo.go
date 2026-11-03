package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type NoteRepository struct {
	db *sqlx.DB
}

func NewNoteRepository(
	db *sqlx.DB,
) *NoteRepository {
	return &NoteRepository{
		db: db,
	}
}

// GetNote mengembalikan catatan tiket di antrian ini. Tiket yang bukan milik antrian ini: ErrNotFound.
// Tiket tanpa catatan: TicketNote dengan Note null (bukan error).
func (r *NoteRepository) GetNote(
	ctx context.Context,
	queueID,
	ticketID int64,
) (model.TicketNote, error) {

	var note model.TicketNote

	const query = `
		SELECT
			t.id AS ticket_id,
			n.note,
			n.created_at,
			n.updated_at
		FROM tickets t
		LEFT JOIN ticket_notes n ON n.ticket_id = t.id
		WHERE t.id = $1 AND t.queue_id = $2
	`

	err := r.db.GetContext(ctx, &note, query, ticketID, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TicketNote{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.TicketNote{}, err
	}
	return note, nil
}

// UpsertNote membuat atau menimpa catatan tiket dalam satu statement (atomik, tahan klik ganda).
// Tiket yang bukan milik antrian ini tidak menghasilkan baris: ErrNotFound.
func (r *NoteRepository) UpsertNote(
	ctx context.Context,
	queueID,
	ticketID,
	userID int64,
	note string,
) (model.UpsertedNote, error) {

	var saved model.UpsertedNote

	// xmax = 0 hanya untuk baris yang baru disisipkan; baris yang ditimpa oleh ON CONFLICT punya xmax > 0.
	const query = `
		INSERT INTO ticket_notes (
			ticket_id, 
			note, 
			created_by, 
			updated_by
		)
		SELECT 
			t.id, 
			$3, 
			$4, 
			$4
		FROM tickets t
		WHERE t.id = $1 AND t.queue_id = $2
		ON CONFLICT (ticket_id) DO UPDATE
			SET note = EXCLUDED.note,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()
		RETURNING
			ticket_id,
			note,
			created_at,
			updated_at,
			(xmax = 0) AS created
	`

	err := r.db.GetContext(ctx, &saved, query, ticketID, queueID, note, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UpsertedNote{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.UpsertedNote{}, err
	}
	return saved, nil
}
