package router

import "github.com/gin-gonic/gin"

// registerCounterRoutes: CRUD counter antrian (owner saja, pengecekan di service).
func registerCounterRoutes(
	rg *gin.RouterGroup,
	counterH CounterHandler,
) {
	// daftar counter antrian
	rg.GET("/:queue_id/counters", counterH.List)
	// buat counter baru
	rg.POST("/:queue_id/counters", counterH.Create)
	// ganti nama counter tertentu
	rg.PUT("/:queue_id/counters/:counter_id", counterH.Rename)
	// hapus counter tertentu
	rg.DELETE("/:queue_id/counters/:counter_id", counterH.Delete)
}
