package metrics

import "github.com/gin-gonic/gin"

// QR memegang metrik qr-service: hanya metrik HTTP umum dan runtime (tidak ada metrik bisnis).
type QR struct {
	*Metrics
}

// NewQR membuat Metrics qr-service.
func NewQR() *QR { return &QR{Metrics: New("qr-service")} }

// Middleware mengembalikan middleware HTTP umum.
func (q *QR) Middleware() gin.HandlerFunc { return q.HTTPMiddleware(nil) }
