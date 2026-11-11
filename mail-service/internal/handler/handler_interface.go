package handler

import "net/http"

// MailHandlerInterface adalah kontrak handler kirim email yang dipakai oleh router.
type MailHandlerInterface interface {
	// Send mengirim satu email lewat body JSON {to, template, data}.
	Send(w http.ResponseWriter, r *http.Request)
}

// SystemHandlerInterface adalah kontrak handler health check dan 404 yang dipakai oleh router.
type SystemHandlerInterface interface {
	// Health menjawab liveness untuk healthcheck kontainer.
	Health(w http.ResponseWriter, r *http.Request)
	// NotFound menjawab rute yang tidak dikenal.
	NotFound(w http.ResponseWriter, r *http.Request)
}

// Pastikan implementasi memenuhi kontrak.
var (
	_ MailHandlerInterface   = (*MailHandler)(nil)
	_ SystemHandlerInterface = (*SystemHandler)(nil)
)
