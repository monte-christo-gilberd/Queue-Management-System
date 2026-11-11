package dto

// RegisterRequest: body POST /users/register.
type RegisterRequest struct {
	// Email: alamat email akun; wajib diisi dan tidak boleh mengandung '<' atau '>' (no_html_tags).
	Email string `json:"email" binding:"required,no_html_tags"`
	// Username: nama pengguna akun; wajib diisi dan tidak boleh mengandung '<' atau '>' (no_html_tags).
	Username string `json:"username" binding:"required,no_html_tags"`
	// Password: password akun; wajib diisi. Panjang 8 sampai 72 karakter diperiksa di service.
	Password string `json:"password" binding:"required"`
}

// LoginRequest: body login. Menerima `identifier` (email atau username); `email` tetap diterima sebagai alias lama.
type LoginRequest struct {
	// Identifier: email atau username; opsional (bila kosong dipakai Email), tanpa '<' atau '>' (no_html_tags).
	Identifier string `json:"identifier" binding:"omitempty,no_html_tags"`
	// Email: alias lama untuk Identifier; opsional, tanpa '<' atau '>' (no_html_tags).
	Email string `json:"email" binding:"omitempty,no_html_tags"`
	// Password: password akun; wajib diisi.
	Password string `json:"password" binding:"required"`
}

// Login mengembalikan identifier yang dipakai: `identifier` jika ada, kalau tidak `email`.
func (r LoginRequest) Login() string {
	// field identifier diutamakan bila diisi
	if r.Identifier != "" {
		return r.Identifier
	}
	// identifier kosong: pakai alias lama email (bisa kosong juga, dicek oleh pemanggil)
	return r.Email
}

// AuthResponse: isi `data` respons login dan registrasi.
type AuthResponse struct {
	// Token: token login (JWT) yang dipakai klien di header Authorization.
	Token string `json:"token"`
}

// ForgotPasswordRequest: body POST /users/forgot-password.
type ForgotPasswordRequest struct {
	// Email: email akun yang mau direset; wajib, maksimal 254 karakter, tanpa '<' atau '>' (no_html_tags).
	Email string `json:"email" binding:"required,max=254,no_html_tags"`
}

// ResetPasswordRequest: body POST /users/reset-password.
type ResetPasswordRequest struct {
	// Token: token reset dari tautan email; wajib, maksimal 200 karakter.
	Token string `json:"token" binding:"required,max=200"`
	// Password: password baru; wajib diisi. Panjang 8 sampai 72 karakter diperiksa di service.
	Password string `json:"password" binding:"required"`
}

// VerifyEmailRequest: body POST /users/verify-email.
type VerifyEmailRequest struct {
	// Token: token verifikasi dari tautan email; wajib, maksimal 200 karakter.
	Token string `json:"token" binding:"required,max=200"`
}

// GoogleLoginRequest: body POST /users/google. id_token adalah ID token yang didapat klien dari Google.
type GoogleLoginRequest struct {
	// IDToken: ID token Google; wajib, maksimal 8192 karakter.
	IDToken string `json:"id_token" binding:"required,max=8192"`
}

// LinkGoogleRequest: body POST /users/google/link. link_token berasal dari respons 409 GOOGLE_LINK_REQUIRED.
type LinkGoogleRequest struct {
	// LinkToken: token penautan dari respons 409; wajib, maksimal 2048 karakter.
	LinkToken string `json:"link_token" binding:"required,max=2048"`
	// Password: password akun yang sudah terdaftar, sebagai bukti kepemilikan; wajib diisi.
	Password string `json:"password" binding:"required"`
}
