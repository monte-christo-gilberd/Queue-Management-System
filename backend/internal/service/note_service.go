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

// NoteRepository: penyimpanan catatan per tiket. Dipenuhi repository (note_repo.go).
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
	// repository: penyimpanan catatan per tiket (NoteRepository).
	repository NoteRepository
	// guard: memastikan requester adalah owner antrian. Nil = fail closed.
	guard QueueGuard
}

// NewNoteService membuat NoteService dengan repository. Guard owner dipasang sesudahnya lewat WithQueueGuard.
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
	// Guard belum dipasang: tolak operasi (fail closed) agar tidak ada yang lolos tanpa pengecekan owner.
	if s.guard == nil {
		return model.TicketNote{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; jika bukan, guard mengembalikan error dan proses dihentikan.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.TicketNote{}, err
	}
	// Ambil catatan tiket dari repository; tiket tanpa catatan menghasilkan Note null, bukan error.
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
	// Guard belum dipasang: tolak operasi (fail closed) agar tidak ada yang lolos tanpa pengecekan owner.
	if s.guard == nil {
		return model.UpsertedNote{}, errNoQueueGuard
	}
	// Pastikan requester adalah owner antrian; jika bukan, guard mengembalikan error dan proses dihentikan.
	if _, err := s.guard.GetQueue(ctx, queueID, requesterID); err != nil {
		return model.UpsertedNote{}, err
	}

	// Buang spasi di awal dan akhir catatan sebelum diperiksa dan disimpan.
	note = strings.TrimSpace(note)
	// Catatan kosong atau lebih dari MaxNoteLength karakter (rune) tidak valid: tolak tanpa menyentuh catatan lama.
	if note == "" || utf8.RuneCountInString(note) > MaxNoteLength {
		return model.UpsertedNote{}, apperror.ErrInvalidNote
	}

	// Buat catatan baru bila belum ada, atau timpa catatan lama bila sudah ada.
	return s.repository.UpsertNote(ctx, queueID, ticketID, requesterID, note)
}
