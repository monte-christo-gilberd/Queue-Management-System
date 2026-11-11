package dto

// IssueTokenRequest: body untuk membuat token join.
type IssueTokenRequest struct {
	// QueueID: id antrian yang akan dibuatkan token. Harus positif.
	QueueID int64 `json:"queue_id"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// TTLSeconds: masa berlaku yang diminta dalam detik, hanya untuk dynamic. 0 = pakai bawaan.
	TTLSeconds int `json:"ttl_seconds"`
}

// TokenResponse: token join yang dibuat.
type TokenResponse struct {
	// Token: string token join yang siap dipakai di tautan QR.
	Token string `json:"token"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix. Nil untuk token statis.
	ExpiresAt *int64 `json:"expires_at"`
}

// VerifyTokenRequest: body untuk memeriksa token join.
type VerifyTokenRequest struct {
	// Token: token join yang akan diperiksa.
	Token string `json:"token"`
}

// VerifyTokenResponse: hasil pemeriksaan token join yang valid.
type VerifyTokenResponse struct {
	// Valid: selalu true pada respons sukses; respons galat dibalas dengan kode error.
	Valid bool `json:"valid"`
	// QueueID: id antrian yang tertulis di dalam token.
	QueueID int64 `json:"queue_id"`
	// Type: jenis token: static atau dynamic.
	Type string `json:"type"`
	// ExpiresAt: waktu kedaluwarsa dalam detik Unix. Nil untuk token statis.
	ExpiresAt *int64 `json:"expires_at"`
}
