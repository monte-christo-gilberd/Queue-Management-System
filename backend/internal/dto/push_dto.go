package dto

// PushSubscribeRequest: body POST /push/subscribe (bentuk PushSubscription.toJSON() di browser).
type PushSubscribeRequest struct {
	// Endpoint: alamat layanan push milik browser. Dipakai sebagai identitas langganan.
	Endpoint string `json:"endpoint" binding:"required"`
	// Keys: kunci enkripsi langganan dari browser.
	Keys struct {
		// P256dh: kunci publik ECDH (base64url) untuk mengenkripsi isi pesan push.
		P256dh string `json:"p256dh" binding:"required"`
		// Auth: secret autentikasi (base64url) yang dipakai bersama P256dh.
		Auth string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
}

// PushUnsubscribeRequest: body untuk berhenti berlangganan push, identifikasinya lewat endpoint browser.
type PushUnsubscribeRequest struct {
	// Endpoint: alamat layanan push langganan yang akan dihentikan.
	Endpoint string `json:"endpoint" binding:"required"`
}
