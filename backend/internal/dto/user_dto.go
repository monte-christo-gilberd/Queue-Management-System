package dto

import (
	"time"

	"qms/internal/model"
)

// MeResponse: profil user yang sedang login. is_owner = punya baris di tabel owners.
// subscription_expires_at hanya ada untuk owner (null untuk customer).
type MeResponse struct {
	// ID: id user di database.
	ID int64 `json:"id"`
	// Email: alamat email akun.
	Email string `json:"email"`
	// Username: nama tampilan akun.
	Username string `json:"username"`
	// IsOwner: true bila user punya antrian (baris di tabel owners).
	IsOwner bool `json:"is_owner"`
	// SubscriptionExpiresAt: akhir masa langganan owner. Null untuk customer.
	SubscriptionExpiresAt *time.Time `json:"subscription_expires_at"`
	// AvatarUpdatedAt: null bila belum ada foto; dipakai klien sebagai penanda ada foto dan pemecah cache.
	AvatarUpdatedAt *time.Time `json:"avatar_updated_at"`
	// EmailNotifications: sakelar email notifikasi tiket.
	EmailNotifications bool `json:"email_notifications"`
	// EmailVerified: false = klien menampilkan banner verifikasi di Profil. Selalu true bila verifikasi dimatikan.
	EmailVerified bool `json:"email_verified"`
}

// UpdateMeRequest: PUT /users/me. Panjang diperiksa service setelah trim.
type UpdateMeRequest struct {
	// Username: nama tampilan baru. Wajib diisi dan tidak boleh berisi tag HTML.
	Username string `json:"username" binding:"required,no_html_tags"`
}

// UpdatePreferencesRequest: PUT /users/me/preferences. Pointer agar "false" dibedakan dari field yang hilang.
type UpdatePreferencesRequest struct {
	// EmailNotifications: nilai baru sakelar email notifikasi. Wajib dikirim; nil berarti field tidak ada.
	EmailNotifications *bool `json:"email_notifications" binding:"required"`
}

// NewMeResponse membentuk respons dari model; satu tempat agar field tidak terlewat.
func NewMeResponse(p model.Profile) MeResponse {
	// Salin data user dan status langganan/foto/verifikasi dari model.Profile ke respons.
	return MeResponse{
		ID:       p.User.ID,
		Email:    p.User.Email,
		Username: p.User.Username,
		IsOwner:  p.IsOwner,
		// Waktu langganan sudah pointer di model: null untuk customer, karena hanya owner yang punya langganan.
		SubscriptionExpiresAt: p.SubscriptionExpiresAt,
		// Waktu foto terakhir sudah pointer di model: null bila belum ada foto.
		AvatarUpdatedAt:    p.AvatarUpdatedAt,
		EmailNotifications: p.User.EmailNotifications,
		EmailVerified:      p.EmailVerified,
	}
}
