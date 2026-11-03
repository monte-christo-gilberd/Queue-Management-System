package router

import "github.com/gin-gonic/gin"

// registerHistoryRoutes: riwayat antrian owner (hanya antrian yang sudah ditutup). Kepemilikan dicek di query.
func registerHistoryRoutes(
	rg *gin.RouterGroup,
	historyH HistoryHandler,
) {
	rg.GET("/queues", historyH.ListQueues)
	rg.GET("/queues/:queue_id", historyH.GetQueue)
}
