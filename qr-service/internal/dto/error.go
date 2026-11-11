package dto

// ErrorBody: isi error di dalam respons: kode tetap dan pesan yang aman ditampilkan.
type ErrorBody struct {
	// Code: kode error tetap, mis. BAD_REQUEST, QR_EXPIRED, atau UNAUTHORIZED.
	Code string `json:"code"`
	// Message: penjelasan singkat untuk klien, tanpa detail teknis internal.
	Message string `json:"message"`
}

// ErrorResponse: bentuk respons error yang dikirim semua endpoint.
type ErrorResponse struct {
	// Error: isi error (kode dan pesan).
	Error ErrorBody `json:"error"`
}
