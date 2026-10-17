package dto

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,no_html_tags"`
	Username string `json:"username" binding:"required,no_html_tags"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest menerima email ATAU username lewat `identifier`.
// Field `email` tetap diterima sebagai alias supaya klien lama tidak rusak.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"omitempty,no_html_tags"`
	Email      string `json:"email" binding:"omitempty,no_html_tags"`
	Password   string `json:"password" binding:"required"`
}

// Login mengembalikan identifier yang dipakai: `identifier` jika ada, kalau tidak `email`.
func (r LoginRequest) Login() string {
	if r.Identifier != "" {
		return r.Identifier
	}
	return r.Email
}

type AuthResponse struct {
	Token string `json:"token"`
}

// MeResponse: profil user yang sedang login. is_owner = punya baris di tabel owners.
type MeResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	IsOwner  bool   `json:"is_owner"`
}
