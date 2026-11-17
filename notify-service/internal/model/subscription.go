package model

// Jenis langganan push. Menentukan pengirim yang dipakai dan tabel asal langganan.
const (
	// KindWebPush: langganan Web Push browser (tabel push_subscriptions). Nilai kosong juga dianggap Web Push.
	KindWebPush = "webpush"
	// KindFCM: token FCM perangkat Android (tabel push_devices).
	KindFCM = "fcm"
)

// Subscription: satu langganan push milik satu perangkat. Web Push memakai Endpoint, P256dh, dan Auth;
// FCM hanya memakai Token.
type Subscription struct {
	// Kind: jenis langganan (KindWebPush atau KindFCM). Kosong berarti Web Push.
	Kind string
	// Endpoint: alamat layanan push milik browser (Web Push).
	Endpoint string
	// P256dh: kunci publik ECDH langganan (base64url, Web Push).
	P256dh string
	// Auth: secret autentikasi langganan (base64url, Web Push).
	Auth string
	// Token: token FCM perangkat (FCM).
	Token string
}

// Channel mengembalikan jenis langganan yang sudah dinormalkan: KindFCM atau KindWebPush.
func (s Subscription) Channel() string {
	// hanya FCM yang ditandai eksplisit; selain itu langganan Web Push
	if s.Kind == KindFCM {
		return KindFCM
	}
	return KindWebPush
}
