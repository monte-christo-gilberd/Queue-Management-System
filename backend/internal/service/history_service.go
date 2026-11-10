package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
)

// Batas jumlah baris per halaman untuk tiap jenis riwayat. Nilai bawaan dipakai bila klien tidak mengirim limit,
// dan nilai maksimal membatasi beban query.
const (
	DefaultHistoryQueuesLimit  = 20
	MaxHistoryQueuesLimit      = 100
	DefaultHistoryTicketsLimit = 50
	MaxHistoryTicketsLimit     = 200
	DefaultMyHistoryLimit      = 20
	MaxMyHistoryLimit          = 100
)

// HistoryRepository: pembacaan riwayat dari tabel live. Semua query memfilter kepemilikan dan status
// (hanya antrian yang sudah ditutup). Dipenuhi repository (history_repo.go).
type HistoryRepository interface {
	ListClosedQueues(
		ctx context.Context,
		ownerID int64,
		after *model.HistoryQueueCursor,
		limit int,
	) ([]model.HistoryQueue, error)
	GetClosedQueue(
		ctx context.Context,
		ownerID,
		queueID int64,
	) (model.HistoryQueue, error)
	ListQueueTickets(
		ctx context.Context,
		queueID int64,
		afterNumber,
		limit int,
	) ([]model.HistoryTicket, error)
	ListUserTickets(
		ctx context.Context,
		userID int64,
		after *model.HistoryQueueCursor,
		limit int,
	) ([]model.UserHistoryTicket, error)
}

// HistoryQueuesPage: satu halaman daftar riwayat. NextCursor kosong = tidak ada halaman berikutnya.
type HistoryQueuesPage struct {
	// Queues: ringkasan antrian di halaman ini.
	Queues []model.HistoryQueue
	// NextCursor: kursor halaman berikutnya. Kosong = tidak ada halaman berikutnya.
	NextCursor string
}

// MyHistoryPage: satu halaman riwayat tiket customer. NextCursor kosong = tidak ada halaman berikutnya.
type MyHistoryPage struct {
	// Tickets: tiket riwayat di halaman ini.
	Tickets []model.UserHistoryTicket
	// NextCursor: kursor halaman berikutnya. Kosong = tidak ada halaman berikutnya.
	NextCursor string
}

// HistoryQueueDetail: ringkasan antrian dan satu halaman nomornya.
type HistoryQueueDetail struct {
	// Queue: ringkasan antrian yang dibuka.
	Queue model.HistoryQueue
	// Tickets: satu halaman nomor di antrian ini.
	Tickets []model.HistoryTicket
	// NextCursor: kursor halaman nomor berikutnya. Kosong = tidak ada halaman berikutnya.
	NextCursor string
}

// HistoryService: riwayat antrian owner (4.15). Hanya antrian yang sudah ditutup yang tampil; antrian aktif
// tetap di dashboard dan daftar antrian. Data dibaca langsung dari tabel live (D1), dan kepemilikan dicek
// di query, jadi non-owner mendapat daftar kosong atau ErrNotFound.
type HistoryService struct {
	// repository: pembacaan riwayat (HistoryRepository).
	repository HistoryRepository
}

// NewHistoryService membuat HistoryService dengan repository riwayat.
func NewHistoryService(repository HistoryRepository) *HistoryService {
	return &HistoryService{repository: repository}
}

// ListQueues mengembalikan halaman daftar antrian yang sudah ditutup. Kursor tidak valid: ErrInvalidCursor.
func (s *HistoryService) ListQueues(
	ctx context.Context,
	ownerID int64,
	cursor string,
	limit int,
) (HistoryQueuesPage, error) {
	// Batasi limit ke rentang yang diizinkan: nol atau negatif memakai bawaan, di atas maksimum dipotong ke maksimum.
	limit = clampLimit(limit, DefaultHistoryQueuesLimit, MaxHistoryQueuesLimit)
	// Uraikan kursor menjadi posisi keyset (closed_at, id); kursor kosong berarti halaman pertama.
	after, err := decodeQueueCursor(cursor)
	// gagal: kursor tidak valid, kembalikan error ke pemanggil.
	if err != nil {
		return HistoryQueuesPage{}, err
	}

	// Ambil satu baris lebih untuk tahu apakah masih ada halaman berikutnya.
	rows, err := s.repository.ListClosedQueues(ctx, ownerID, after, limit+1)
	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return HistoryQueuesPage{}, err
	}
	page := HistoryQueuesPage{Queues: rows}
	// Jika baris ekstra ikut terbaca, masih ada halaman berikutnya: potong hasil ke limit.
	if len(rows) > limit {
		page.Queues = rows[:limit]
		last := page.Queues[limit-1]
		// Buat kursor dari closed_at dan id baris terakhir halaman ini.
		page.NextCursor = encodeQueueCursor(last.ClosedAt, last.ID)
	}
	return page, nil
}

// ListMyHistory mengembalikan riwayat tiket milik user (customer), terbaru dulu, per halaman. Hanya tiket user itu
// sendiri; tidak ada parameter user lain, jadi tidak ada jalan membaca tiket orang lain. Kursor tidak valid:
// ErrInvalidCursor.
func (s *HistoryService) ListMyHistory(
	ctx context.Context,
	userID int64,
	cursor string,
	limit int,
) (MyHistoryPage, error) {
	// Batasi limit ke rentang yang diizinkan: nol atau negatif memakai bawaan, di atas maksimum dipotong ke maksimum.
	limit = clampLimit(limit, DefaultMyHistoryLimit, MaxMyHistoryLimit)
	// Uraikan kursor menjadi posisi keyset (waktu, id); kursor kosong berarti halaman pertama.
	after, err := decodeQueueCursor(cursor)
	// gagal: kursor tidak valid, kembalikan error ke pemanggil.
	if err != nil {
		return MyHistoryPage{}, err
	}

	// Ambil tiket user hingga limit+1 baris, untuk mendeteksi halaman berikutnya.
	rows, err := s.repository.ListUserTickets(ctx, userID, after, limit+1)
	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return MyHistoryPage{}, err
	}
	page := MyHistoryPage{Tickets: rows}
	// Jika baris ekstra ikut terbaca, masih ada halaman berikutnya: potong hasil ke limit.
	if len(rows) > limit {
		page.Tickets = rows[:limit]
		last := page.Tickets[limit-1]
		// Kursor memakai pasangan (waktu bergabung, id tiket): sama bentuknya dengan kursor daftar riwayat owner.
		page.NextCursor = encodeQueueCursor(last.JoinedAt, last.TicketID)
	}
	return page, nil
}

// GetQueue mengembalikan ringkasan dan satu halaman nomor antrian yang sudah ditutup. Bukan milik owner,
// masih aktif, atau tidak ada: ErrNotFound.
func (s *HistoryService) GetQueue(
	ctx context.Context,
	ownerID,
	queueID int64,
	cursor string,
	limit int,
) (HistoryQueueDetail, error) {
	// Batasi limit ke rentang yang diizinkan: nol atau negatif memakai bawaan, di atas maksimum dipotong ke maksimum.
	limit = clampLimit(limit, DefaultHistoryTicketsLimit, MaxHistoryTicketsLimit)
	// Uraikan kursor nomor tiket menjadi posisi awal halaman; kursor kosong berarti mulai dari nomor 0.
	after, err := decodeTicketCursor(cursor)
	// gagal: kursor tidak valid, kembalikan error ke pemanggil.
	if err != nil {
		return HistoryQueueDetail{}, err
	}

	// Ambil ringkasan antrian; repository menolak jika requester bukan owner atau antrian belum ditutup.
	queue, err := s.repository.GetClosedQueue(ctx, ownerID, queueID)
	// gagal (termasuk ErrNotFound): kembalikan error ke pemanggil.
	if err != nil {
		return HistoryQueueDetail{}, err
	}
	// Ambil nomor tiket antrian setelah posisi kursor, hingga limit+1 baris untuk deteksi halaman berikutnya.
	rows, err := s.repository.ListQueueTickets(ctx, queueID, after, limit+1)
	// gagal membaca database: kembalikan error ke pemanggil.
	if err != nil {
		return HistoryQueueDetail{}, err
	}
	detail := HistoryQueueDetail{Queue: queue, Tickets: rows}
	// Jika baris ekstra ikut terbaca, masih ada halaman berikutnya: potong hasil ke limit.
	if len(rows) > limit {
		detail.Tickets = rows[:limit]
		// Kursor berikutnya adalah nomor tiket terakhir di halaman ini, diubah menjadi string.
		detail.NextCursor = strconv.Itoa(detail.Tickets[limit-1].TicketNumber)
	}
	return detail, nil
}

// exportPageSize: jumlah tiket per halaman saat ekspor. Antrian besar dibaca bertahap, bukan sekaligus.
const exportPageSize = 500

// TicketSink menerima nomor antrian per halaman saat ekspor (dipenuhi oleh export.Sink).
type TicketSink interface {
	WriteTickets(tickets []model.HistoryTicket) error
}

// ExportQueue memastikan requester adalah owner antrian yang sudah ditutup, dan mengembalikan ringkasannya
// (untuk nama berkas dan awalan nomor). Bukan owner, masih aktif, atau tidak ada: ErrNotFound.
func (s *HistoryService) ExportQueue(ctx context.Context, ownerID, queueID int64) (model.HistoryQueue, error) {
	// Ambil antrian tertutup; jika requester bukan owner, repository mengembalikan ErrNotFound.
	return s.repository.GetClosedQueue(ctx, ownerID, queueID)
}

// StreamTickets membaca seluruh nomor antrian, urut nomor naik, per halaman, dan menyerahkannya ke sink.
// Dipanggil setelah ExportQueue memeriksa kepemilikan.
func (s *HistoryService) StreamTickets(ctx context.Context, queueID int64, sink TicketSink) error {
	after := 0
	// Ulangi membaca halaman demi halaman sampai habis atau terjadi error.
	for {
		// Baca satu halaman nomor tiket setelah posisi terakhir, sebanyak exportPageSize.
		page, err := s.repository.ListQueueTickets(ctx, queueID, after, exportPageSize)
		// gagal membaca database: kembalikan error ke pemanggil.
		if err != nil {
			return err
		}
		// Halaman kosong berarti semua nomor sudah terbaca: proses selesai.
		if len(page) == 0 {
			return nil
		}
		// Serahkan halaman ini ke sink (misalnya penulis berkas ekspor); gagal berarti hentikan proses.
		if err := sink.WriteTickets(page); err != nil {
			return err
		}
		// Halaman yang tidak penuh berarti ini halaman terakhir: proses selesai.
		if len(page) < exportPageSize {
			return nil
		}
		after = page[len(page)-1].TicketNumber
	}
}

// clampLimit memastikan limit berada dalam rentang yang diizinkan: nol atau negatif memakai def, di atas max
// dipotong ke max.
func clampLimit(limit, def, max int) int {
	// Pilih nilai limit berdasarkan kondisi berurutan di bawah ini.
	switch {
	// Limit nol atau negatif tidak valid: pakai nilai bawaan def.
	case limit <= 0:
		return def
	// Limit di atas batas maksimum: potong ke max.
	case limit > max:
		return max
	// Limit sudah berada dalam rentang: pakai apa adanya.
	default:
		return limit
	}
}

// Kursor daftar antrian: "<closed_at dalam mikrodetik>.<id>". Mikrodetik karena itu presisi Postgres, jadi
// perbandingan keyset tidak meleset.
func encodeQueueCursor(closedAt time.Time, id int64) string {
	// Format kursor: mikrodetik waktu dan id dipisah titik.
	return fmt.Sprintf("%d.%d", closedAt.UnixMicro(), id)
}

func decodeQueueCursor(cursor string) (*model.HistoryQueueCursor, error) {
	// Kursor kosong: tidak ada posisi awal, kembalikan nil agar query mulai dari halaman pertama.
	if cursor == "" {
		return nil, nil
	}
	// Pecah kursor berdasarkan titik menjadi bagian waktu dan id.
	parts := strings.Split(cursor, ".")
	// Kursor harus tepat dua bagian; selain itu formatnya salah.
	if len(parts) != 2 {
		return nil, apperror.ErrInvalidCursor
	}
	// Ubah bagian pertama (mikrodetik) menjadi angka int64.
	micro, err1 := strconv.ParseInt(parts[0], 10, 64)
	// Ubah bagian kedua (id) menjadi angka int64.
	id, err2 := strconv.ParseInt(parts[1], 10, 64)
	// gagal parse, atau id tidak positif: kursor dianggap tidak valid.
	if err1 != nil || err2 != nil || id <= 0 {
		return nil, apperror.ErrInvalidCursor
	}
	// Ubah mikrodetik menjadi waktu UTC dan kembalikan sebagai posisi keyset.
	return &model.HistoryQueueCursor{ClosedAt: time.UnixMicro(micro).UTC(), ID: id}, nil
}

// Kursor daftar nomor: nomor tiket terakhir yang sudah dikirim.
func decodeTicketCursor(cursor string) (int, error) {
	// Kursor kosong: mulai dari awal, yaitu posisi nol.
	if cursor == "" {
		return 0, nil
	}
	// Ubah kursor string menjadi nomor tiket integer.
	n, err := strconv.Atoi(cursor)
	// gagal parse, atau nomor negatif: kursor dianggap tidak valid.
	if err != nil || n < 0 {
		return 0, apperror.ErrInvalidCursor
	}
	return n, nil
}
