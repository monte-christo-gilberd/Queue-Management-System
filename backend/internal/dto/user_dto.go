package dto

import (
	"time"

	"qms/internal/model"
)

// RegisterRequest: body POST untuk registrasi akun. Email dan username tidak boleh berisi tag HTML.
type RegisterRequest struct {
	// Email: alamat email akun baru. Wajib diisi.
	Email string `json:"email" binding:"required,no_html_tags"`
	// Username: nama tampilan akun baru. Wajib diisi.
	Username string `json:"username" binding:"required,no_html_tags"`
	// Password: password akun baru. Panjangnya diperiksa service.
	Password string `json:"password" binding:"required"`
}

// LoginRequest: body login. Menerima email ATAU username lewat Identifier. Field Email tetap diterima sebagai
// alias supaya klien lama tidak rusak.
type LoginRequest struct {
	// Identifier: email atau username. Dipakai bila diisi.
	Identifier string `json:"identifier" binding:"omitempty,no_html_tags"`
	// Email: alias lama untuk Identifier. Dipakai hanya bila Identifier kosong.
	Email string `json:"email" binding:"omitempty,no_html_tags"`
	// Password: password akun.
	Password string `json:"password" binding:"required"`
}

// Login mengembalikan identifier yang dipakai: `identifier` jika ada, kalau tidak `email`.
func (r LoginRequest) Login() string {
	// Klien baru mengirim identifier: pakai itu.
	if r.Identifier != "" {
		return r.Identifier
	}
	// Klien lama hanya mengirim email: pakai field email sebagai alias.
	return r.Email
}

// AuthResponse: respons login dan registrasi, berisi token JWT.
type AuthResponse struct {
	// Token: JWT yang dipakai klien di header Authorization.
	Token string `json:"token"`
}

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

// ForgotPasswordRequest: POST /users/forgot-password. Respons selalu sama (202) apa pun isinya, selama formatnya sah.
type ForgotPasswordRequest struct {
	// Email: alamat email akun yang lupa password. Maksimal 254 karakter.
	Email string `json:"email" binding:"required,max=254,no_html_tags"`
}

// VerifyEmailRequest: POST /users/verify-email. Token dari tautan di email.
type VerifyEmailRequest struct {
	// Token: token verifikasi dari tautan di email. Maksimal 200 karakter.
	Token string `json:"token" binding:"required,max=200"`
}

// ResetPasswordRequest: POST /users/reset-password. Panjang password diperiksa service.
type ResetPasswordRequest struct {
	// Token: token reset password dari tautan di email. Maksimal 200 karakter.
	Token string `json:"token" binding:"required,max=200"`
	// Password: password baru. Panjangnya diperiksa service.
	Password string `json:"password" binding:"required"`
}
