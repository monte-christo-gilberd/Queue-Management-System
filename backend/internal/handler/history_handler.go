package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/export"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/gin-gonic/gin"
)

// HistoryService adalah logika riwayat antrian dan tiket (dipenuhi oleh service).
type HistoryService interface {
	// ListQueues mengambil daftar antrian milik owner yang sudah ditutup, per halaman dengan cursor.
	ListQueues(
		ctx context.Context,
		ownerID int64,
		cursor string,
		limit int,
	) (service.HistoryQueuesPage, error)
	// GetQueue mengambil ringkasan dan nomor satu antrian yang sudah ditutup, hanya untuk owner-nya.
	GetQueue(
		ctx context.Context,
		ownerID,
		queueID int64,
		cursor string,
		limit int,
	) (service.HistoryQueueDetail, error)
	// ListMyHistory mengambil riwayat tiket milik user yang login, per halaman dengan cursor.
	ListMyHistory(
		ctx context.Context,
		userID int64,
		cursor string,
		limit int,
	) (service.MyHistoryPage, error)
	exportService
}

// exportService: bagian HistoryService untuk ekspor.
type exportService interface {
	// ExportQueue mengambil data antrian untuk ekspor, hanya untuk owner-nya.
	ExportQueue(ctx context.Context, ownerID, queueID int64) (model.HistoryQueue, error)
	// StreamTickets mengirim semua tiket antrian ke sink, halaman demi halaman.
	StreamTickets(ctx context.Context, queueID int64, sink service.TicketSink) error
}

// HistoryHandler menangani rute riwayat antrian dan ekspornya.
type HistoryHandler struct {
	// service berisi logika riwayat.
	service HistoryService
}

// NewHistoryHandler membuat HistoryHandler dengan service yang diberikan.
func NewHistoryHandler(service HistoryService) *HistoryHandler {
	// isi handler dengan service
	return &HistoryHandler{service: service}
}

// ListQueues: GET /api/v1/history/queues?cursor=&limit= — antrian milik user yang sudah ditutup, terbaru dulu.
// User yang bukan owner mendapat daftar kosong.
func (h *HistoryHandler) ListQueues(c *gin.Context) {
	// baca dan periksa parameter limit; bila tidak valid, respons error sudah dikirim
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	// ambil halaman antrian untuk owner yang login, dengan cursor dari query
	page, err := h.service.ListQueues(c.Request.Context(), c.GetInt64("user_id"), c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	// kirim daftar antrian beserta cursor halaman berikutnya
	respondData(c, http.StatusOK, dto.NewHistoryQueuesResponse(page.Queues, page.NextCursor))
}

// ListMine: GET /api/v1/users/me/history?cursor=&limit= — riwayat tiket milik user yang login (customer), satu
// baris per tiket, terbaru dulu. Tidak ada parameter user: hanya tiket milik token yang dikembalikan.
func (h *HistoryHandler) ListMine(c *gin.Context) {
	// baca dan periksa parameter limit; bila tidak valid, respons error sudah dikirim
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	// ambil halaman riwayat tiket milik user yang login
	page, err := h.service.ListMyHistory(c.Request.Context(), c.GetInt64("user_id"), c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	// kirim daftar tiket beserta cursor halaman berikutnya
	respondData(c, http.StatusOK, dto.NewMyHistoryResponse(page.Tickets, page.NextCursor))
}

// GetQueue: GET /api/v1/history/queues/:queue_id?cursor=&limit= — ringkasan dan nomor satu antrian yang sudah
// ditutup. Bukan milik owner atau masih aktif: 404.
func (h *HistoryHandler) GetQueue(c *gin.Context) {
	// ubah queue_id dari URL menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// baca dan periksa parameter limit; bila tidak valid, respons error sudah dikirim
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	// ambil ringkasan dan halaman tiket antrian untuk owner yang login
	detail, err := h.service.GetQueue(c.Request.Context(), c.GetInt64("user_id"), queueID, c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	// kirim ringkasan antrian, daftar tiket, dan cursor halaman berikutnya
	respondData(c, http.StatusOK, dto.NewHistoryQueueDetailResponse(detail.Queue, detail.Tickets, detail.NextCursor))
}

// lazyDownload menunda header unduhan sampai byte pertama ditulis. Selama belum ada byte yang keluar, kegagalan
// (mis. database) masih bisa dibalas sebagai JSON 500 biasa, bukan berkas yang rusak.
type lazyDownload struct {
	// c adalah konteks Gin tempat byte dan header ditulis.
	c *gin.Context
	// contentType adalah MIME type berkas (diset saat byte pertama ditulis).
	contentType string
	// filename adalah nama berkas unduhan (dipakai di Content-Disposition).
	filename string
	// started menandai bahwa header dan status 200 sudah dikirim, sehingga tidak dikirim dua kali.
	started bool
}

// Write menerima byte dari sink. Pada panggilan pertama, ia menulis header unduhan dan status 200 lebih dulu.
// Setelah itu, byte diteruskan ke respons.
func (l *lazyDownload) Write(p []byte) (int, error) {
	// panggilan pertama: kirim header unduhan dan status 200 sebelum byte pertama
	if !l.started {
		// tandai sudah dimulai agar header tidak dikirim lagi
		l.started = true
		// ambil header respons untuk diisi
		h := l.c.Writer.Header()
		// tipe isi dan nama berkas unduhan
		h.Set("Content-Type", l.contentType)
		h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, l.filename))
		// jangan di-cache, dan jangan ditebak tipenya oleh browser
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		// status 200 OK
		l.c.Status(http.StatusOK)
	}
	// teruskan byte ke respons
	return l.c.Writer.Write(p)
}

// Export: GET /api/v1/history/queues/:queue_id/export?format=csv|xlsx — unduh seluruh nomor antrian yang sudah
// ditutup. Owner saja; bukan owner atau masih aktif: 404. Format wajib diisi; selain csv atau xlsx: 400.
func (h *HistoryHandler) Export(c *gin.Context) {
	// ubah queue_id dari URL menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// format harus csv atau xlsx; selain itu tolak
	format, ok := export.ParseFormat(c.Query("format"))
	if !ok {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "format must be csv or xlsx")
		return
	}
	// ambil data antrian untuk ekspor (sekaligus memastikan user adalah owner dan antrian sudah ditutup)
	queue, err := h.service.ExportQueue(c.Request.Context(), c.GetInt64("user_id"), queueID)
	if err != nil {
		respondHistoryError(c, err)
		return
	}

	// siapkan penulis unduhan yang baru mengirim header saat byte pertama ditulis
	out := &lazyDownload{
		c:           c,
		contentType: format.ContentType(),
		filename:    export.FileName(queue.QueueName, queue.ID, format),
	}
	// buat sink untuk format yang diminta; dd berarti sink tidak bisa dibuat
	sink, err := export.NewSink(format, out, queue)
	// bila sink berhasil dibuat, alirkan semua tiket ke sink
	if err == nil {
		err = h.service.StreamTickets(c.Request.Context(), queueID, sink)
	}
	// bila tidak ada dd sejauh ini, selesaikan berkas (XLSX baru menulis byte di Close)
	if err == nil {
		err = sink.Close()
	}
	// ada dd
	if err != nil {
		// belum ada byte yang terkirim: balas JSON error biasa
		if !out.started {
			respondHistoryError(c, err)
			return
		}
		// Byte sudah terkirim: status tidak bisa diubah lagi. Catat dan hentikan; berkas terpotong.
		_ = c.Error(err)
		c.Abort()
	}
}

// historyLimit membaca ?limit=. Kosong berarti 0 (service memakai bawaan); bukan angka: 400.
func historyLimit(c *gin.Context) (int, bool) {
	// ambil nilai limit dari query
	raw := c.Query("limit")
	// tidak diisi: 0 berarti service memakai nilai bawaannya
	if raw == "" {
		return 0, true
	}
	// ubah menjadi angka; gagal atau negatif berarti tidak valid
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid limit")
		return 0, false
	}
	// limit valid
	return limit, true
}

// respondHistoryError menerjemahkan dd dari service riwayat menjadi respons HTTP.
func respondHistoryError(c *gin.Context, err error) {
	// pilih respons sesuai jenis dd
	switch {
	// antrian tidak ditemukan, bukan milik user, atau belum ditutup: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
	// cursor halaman tidak valid: 400
	case errors.Is(err, apperror.ErrInvalidCursor):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid cursor")
	// dd lain: 500 tanpa detail
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
