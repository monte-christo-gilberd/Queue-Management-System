package service

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"strings"
	"unicode/utf8"
)

// MaxNoteLength: batas panjang catatan dalam karakter (rune), setelah dipangkas spasi.
const MaxNoteLength = 1000

type NoteRepository interface {
	GetNote(
		ctx context.Context,
		queueID,
		ticketID int64,
	) (model.TicketNote, error)
	UpsertNote(
		ctx context.Context,
		queueID,
		ticketID,
		userID int64,
		note string,
	) (model.UpsertedNote, error)
}

// NoteService: catatan owner per nomor tiket. Hanya owner antrian yang boleh membaca dan menulis.
// Non-owner mendapat apperror.ErrNotFound, sama seperti endpoint antrian lainnya.
type NoteService struct {
	repository NoteRepository
	guard      QueueGuard
}

func NewNoteService(
	repository NoteRepository,
) *NoteService {
	return &NoteService{
		repository: repository,
	}
}

// WithQueueGuard memasang pengecekan owner. Tanpa guard, semua operasi gagal (fail closed).
func (s *NoteService) WithQueueGuard(
	guard QueueGuard,
) *NoteService {
	s.guard = guard
	return s
}

// GetNote mengembalikan catatan tiket. Tiket tanpa catatan: Note null, bukan error.
func (s *NoteService) GetNote(
	ctx context.Context,
	queueID,
	requesterID,
	ticketID int64,
) (model.TicketNote, error) {
	if s.guard == nil {
		return model.TicketNote{}, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.TicketNote{}, err
	}
	return s.repository.GetNote(ctx, queueID, ticketID)
}

// SaveNote membuat catatan bila belum ada, atau menimpanya bila sudah ada. Catatan kosong
// atau melebihi MaxNoteLength ditolak (apperror.ErrInvalidNote) tanpa menyentuh catatan lama.
func (s *NoteService) SaveNote(
	ctx context.Context,
	queueID,
	requesterID,
	ticketID int64,
	note string,
) (model.UpsertedNote, error) {
	if s.guard == nil {
		return model.UpsertedNote{}, errNoQueueGuard
	}
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.UpsertedNote{}, err
	}

	note = strings.TrimSpace(note)
	if note == "" || utf8.RuneCountInString(note) > MaxNoteLength {
		return model.UpsertedNote{}, apperror.ErrInvalidNote
	}

	return s.repository.UpsertNote(ctx, queueID, ticketID, requesterID, note)
}
