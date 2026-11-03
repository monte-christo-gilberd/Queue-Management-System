package router

import "github.com/gin-gonic/gin"

// registerCounterRoutes: CRUD counter antrian (owner saja, pengecekan di service).
func registerCounterRoutes(
	rg *gin.RouterGroup,
	counterH CounterHandler,
) {
	rg.GET("/:queue_id/counters", counterH.List)
	rg.POST("/:queue_id/counters", counterH.Create)
	rg.PUT("/:queue_id/counters/:counter_id", counterH.Rename)
	rg.DELETE("/:queue_id/counters/:counter_id", counterH.Delete)
}
