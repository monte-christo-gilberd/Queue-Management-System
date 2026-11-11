package router

import "net/http"

// MailHandler adalah handler kirim email yang dipakai router.
type MailHandler interface {
	// Send mengirim satu email lewat body JSON {to, template, data}.
	Send(w http.ResponseWriter, r *http.Request)
}

// SystemHandler adalah handler health check dan 404 yang dipakai router.
type SystemHandler interface {
	// Health menjawab liveness untuk healthcheck kontainer.
	Health(w http.ResponseWriter, r *http.Request)
	// NotFound menjawab rute yang tidak dikenal.
	NotFound(w http.ResponseWriter, r *http.Request)
}

// ServiceTokenMiddleware membungkus handler dengan pemeriksaan X-Service-Token.
type ServiceTokenMiddleware func(http.HandlerFunc) http.HandlerFunc
