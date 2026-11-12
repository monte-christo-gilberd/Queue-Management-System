package router

import "github.com/gin-gonic/gin"

// MailHandler adalah handler kirim email yang dipakai router.
type MailHandler interface {
	// Send mengirim satu email lewat body JSON {to, template, data}.
	Send(c *gin.Context)
}

// SystemHandler adalah handler health check dan 404 yang dipakai router.
type SystemHandler interface {
	// Health menjawab liveness untuk healthcheck kontainer.
	Health(c *gin.Context)
	// NotFound menjawab rute yang tidak dikenal.
	NotFound(c *gin.Context)
}
