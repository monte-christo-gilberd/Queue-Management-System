package router

import "github.com/gin-gonic/gin"

// registerHistoryRoutes: riwayat antrian owner (hanya antrian yang sudah ditutup). Kepemilikan dicek di query.
func registerHistoryRoutes(
	rg *gin.RouterGroup,
	historyH HistoryHandler,
) {
	// daftar antrian yang sudah ditutup
	rg.GET("/queues", historyH.ListQueues)
	// riwayat satu antrian
	rg.GET("/queues/:queue_id", historyH.GetQueue)
	// unduh riwayat antrian sebagai CSV atau XLSX
	rg.GET("/queues/:queue_id/export", historyH.Export)
}
