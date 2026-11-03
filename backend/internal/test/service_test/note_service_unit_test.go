package service_test

import (
	"context"
	"database/sql"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memNoteRepo: fake service.NoteRepository. ticketQueue memetakan tiket ke antriannya, seperti kolom tickets.queue_id.
type memNoteRepo struct {
	mu          sync.Mutex
	ticketQueue map[int64]int64
	notes       map[int64]model.TicketNote
	upserts     int
}

func newMemNoteRepo() *memNoteRepo {
	return &memNoteRepo{ticketQueue: map[int64]int64{}, notes: map[int64]model.TicketNote{}}
}

func (m *memNoteRepo) GetNote(_ context.Context, queueID, ticketID int64) (model.TicketNote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q, ok := m.ticketQueue[ticketID]; !ok || q != queueID {
		return model.TicketNote{}, apperror.ErrNotFound
	}
	if n, ok := m.notes[ticketID]; ok {
		return n, nil
	}
	return model.TicketNote{TicketID: ticketID}, nil
}

func (m *memNoteRepo) UpsertNote(_ context.Context, queueID, ticketID, _ int64, note string) (model.UpsertedNote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q, ok := m.ticketQueue[ticketID]; !ok || q != queueID {
		return model.UpsertedNote{}, apperror.ErrNotFound
	}
	m.upserts++
	now := time.Now()
	existing, created := m.notes[ticketID]
	if !created {
		existing = model.TicketNote{TicketID: ticketID, CreatedAt: sql.NullTime{Time: now, Valid: true}}
	}
	existing.Note = sql.NullString{String: note, Valid: true}
	existing.UpdatedAt = sql.NullTime{Time: now, Valid: true}
	m.notes[ticketID] = existing
	return model.UpsertedNote{TicketNote: existing, Created: !created}, nil
}

var _ service.NoteRepository = (*memNoteRepo)(nil)

func newNoteSvc(repo *memNoteRepo) *service.NoteService {
	return service.NewNoteService(repo).WithQueueGuard(fakeQueueGuard{ownerID: 7})
}

func TestNote_GetWithoutNoteReturnsNullNote(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1

	got, err := newNoteSvc(repo).GetNote(context.Background(), 1, 7, 10)

	require.NoError(t, err)
	assert.Equal(t, int64(10), got.TicketID)
	assert.False(t, got.Note.Valid, "belum ada catatan: note null, bukan error")
}

func TestNote_SaveCreatesWhenNoneExists(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1

	saved, err := newNoteSvc(repo).SaveNote(context.Background(), 1, 7, 10, "  pelanggan minta dipanggil ulang  ")

	require.NoError(t, err)
	assert.True(t, saved.Created)
	assert.Equal(t, "pelanggan minta dipanggil ulang", saved.Note.String, "spasi di awal dan akhir dipangkas")
}

func TestNote_SaveEditsWhenNoteExists(t *testing.T) {
	// Alur "ambil dulu, lalu simpan": catatan lama ditimpa di tempat, bukan ditambah.
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1
	svc := newNoteSvc(repo)
	ctx := context.Background()

	_, err := svc.SaveNote(ctx, 1, 7, 10, "pertama")
	require.NoError(t, err)

	existing, err := svc.GetNote(ctx, 1, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, "pertama", existing.Note.String, "GET sebelum edit mengembalikan catatan yang ada")

	edited, err := svc.SaveNote(ctx, 1, 7, 10, "kedua")
	require.NoError(t, err)
	assert.False(t, edited.Created, "simpan kedua untuk tiket yang sama adalah edit")
	assert.Equal(t, "kedua", edited.Note.String)
	assert.Len(t, repo.notes, 1, "satu tiket tetap satu catatan")
}

func TestNote_EmptyOrTooLongIsRejectedWithoutChangingExisting(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1
	svc := newNoteSvc(repo)
	ctx := context.Background()
	_, err := svc.SaveNote(ctx, 1, 7, 10, "asli")
	require.NoError(t, err)

	for _, bad := range []string{"", "   \n\t ", strings.Repeat("a", service.MaxNoteLength+1)} {
		_, err := svc.SaveNote(ctx, 1, 7, 10, bad)
		assert.ErrorIs(t, err, apperror.ErrInvalidNote)
	}

	got, err := svc.GetNote(ctx, 1, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, "asli", got.Note.String, "catatan lama tidak berubah oleh input yang ditolak")
}

func TestNote_LengthCountsCharactersNotBytes(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1
	// 1000 karakter multibyte: lolos batas karakter walau byte-nya lebih dari 1000.
	note := strings.Repeat("é", service.MaxNoteLength)

	_, err := newNoteSvc(repo).SaveNote(context.Background(), 1, 7, 10, note)

	assert.NoError(t, err)
}

func TestNote_NonOwnerIsNotFound(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1
	svc := newNoteSvc(repo)
	ctx := context.Background()

	_, err := svc.GetNote(ctx, 1, 99, 10)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	_, err = svc.SaveNote(ctx, 1, 99, 10, "bukan milik saya")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, repo.upserts, "non-owner tidak menyentuh data")
}

func TestNote_TicketFromAnotherQueueIsNotFound(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 2 // tiket 10 milik antrian 2, bukan 1

	_, err := newNoteSvc(repo).SaveNote(context.Background(), 1, 7, 10, "salah antrian")

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestNote_ConcurrentSavesKeepOneNote(t *testing.T) {
	repo := newMemNoteRepo()
	repo.ticketQueue[10] = 1
	svc := newNoteSvc(repo)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.SaveNote(context.Background(), 1, 7, 10, "klik")
		}()
	}
	wg.Wait()

	assert.Len(t, repo.notes, 1)
	assert.Equal(t, 20, repo.upserts)
}
