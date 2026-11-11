package router

import "net/http"

// registerMailRoutes mendaftarkan endpoint kirim email. Satu-satunya rute yang dilindungi X-Service-Token.
func registerMailRoutes(
	mux *http.ServeMux,
	h MailHandler,
	auth ServiceTokenMiddleware,
) {
	// Endpoint kirim: dibungkus middleware token sehingga token diperiksa dulu.
	mux.HandleFunc("POST /internal/v1/send", auth(h.Send))
}
