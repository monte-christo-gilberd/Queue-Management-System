package dto

// SendRequest: {"to":"budi@contoh.id","template":"test","data":{...}}. Isi email ditentukan templat di
// mail-service; pemanggil hanya memberi nama templat dan datanya.
type SendRequest struct {
	// To: alamat email penerima.
	To string `json:"to"`
	// Template: nama templat email, mis. "test" atau "password-reset".
	Template string `json:"template"`
	// Data: nilai yang mengisi templat. Kunci yang hilang membuat permintaan ditolak.
	Data map[string]any `json:"data"`
}

// SendResponse: isi field "data" saat email berhasil dikirim.
type SendResponse struct {
	// Status: selalu "sent".
	Status string `json:"status"`
}

// HealthResponse: body GET /health (tanpa pembungkus "data").
type HealthResponse struct {
	// Status: selalu "ok" selama proses hidup.
	Status string `json:"status"`
}
