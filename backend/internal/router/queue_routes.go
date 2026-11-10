package router

import "github.com/gin-gonic/gin"

// registerQueueRoutes mendaftarkan rute antrian (buat, daftar, detail, ubah, tutup), QR, pengaturan skip,
// dan operasi tiket. Semua rute ini berada di grup /queues yang sudah memakai middleware login.
func registerQueueRoutes(
	rg *gin.RouterGroup,
	queueH QueueHandler,
	ticketH TicketHandler,
) {
	// buat antrian baru
	rg.POST("", queueH.Create)
	// daftar antrian milik user
	rg.GET("", queueH.List)

	// QR operator: status, mode, dan penggantian kode statis.
	// status dan jenis QR antrian
	rg.GET("/:queue_id/qr", queueH.GetQR)
	// ubah jenis QR antrian
	rg.PUT("/:queue_id/qr/type", queueH.SetQRType)
	// ganti kode QR statis dengan yang baru
	rg.POST("/:queue_id/qr/replace", queueH.ReplaceQR)

	// pengaturan skip antrian: baca dan ubah
	rg.GET("/:queue_id/settings/skip", queueH.GetSkipSettings)
	rg.PUT("/:queue_id/settings/skip", queueH.UpdateSkipSettings)

	// Join berbasis token: nomor id tidak dipakai lagi dari URL.
	// operasi tiket: panggil berikutnya, lewati, panggil ulang, batalkan, hapus, sisipkan, daftar, dan status
	rg.POST("/:queue_id/next", ticketH.Next)
	rg.POST("/:queue_id/skip", ticketH.Skip)
	rg.POST("/:queue_id/recall", ticketH.Recall)
	rg.POST("/:queue_id/cancel", ticketH.Cancel)
	rg.POST("/:queue_id/tickets/:ticket_id/remove", ticketH.Remove)
	rg.POST("/:queue_id/insert", ticketH.Insert)
	rg.GET("/:queue_id/tickets", ticketH.List)
	rg.GET("/:queue_id/status", ticketH.Status)

	// detail, ubah, dan tutup satu antrian
	rg.GET("/:queue_id", queueH.Get)
	rg.PUT("/:queue_id", queueH.Update)
	rg.POST("/:queue_id/close", queueH.Close)
}
