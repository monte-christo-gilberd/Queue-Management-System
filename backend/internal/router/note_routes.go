package router

import "github.com/gin-gonic/gin"

// registerNoteRoutes: catatan per nomor tiket, owner saja (pengecekan di service).
func registerNoteRoutes(
	rg *gin.RouterGroup,
	noteH NoteHandler,
) {
	// baca catatan tiket
	rg.GET("/:queue_id/tickets/:ticket_id/note", noteH.Get)
	// simpan catatan tiket
	rg.PUT("/:queue_id/tickets/:ticket_id/note", noteH.Save)
}
