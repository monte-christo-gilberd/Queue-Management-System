package handler

import "github.com/gin-gonic/gin"

// MailHandlerInterface adalah kontrak handler kirim email yang dipakai oleh router.
type MailHandlerInterface interface {
	// Send mengirim satu email lewat body JSON {to, template, data}.
	Send(c *gin.Context)
}

// SystemHandlerInterface adalah kontrak handler health check dan 404 yang dipakai oleh router.
type SystemHandlerInterface interface {
	// Health menjawab liveness untuk healthcheck kontainer.
	Health(c *gin.Context)
	// NotFound menjawab rute yang tidak dikenal.
	NotFound(c *gin.Context)
}

// Pastikan implementasi memenuhi kontrak.
var (
	_ MailHandlerInterface   = (*MailHandler)(nil)
	_ SystemHandlerInterface = (*SystemHandler)(nil)
)
