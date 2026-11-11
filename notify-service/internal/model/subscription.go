package model

// Subscription: satu langganan push milik browser, yaitu tujuan dan kunci enkripsinya.
type Subscription struct {
	// Endpoint: alamat layanan push milik browser.
	Endpoint string
	// P256dh: kunci publik ECDH langganan (base64url).
	P256dh string
	// Auth: secret autentikasi langganan (base64url).
	Auth string
}
