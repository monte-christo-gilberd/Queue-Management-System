package model

import "time"

// Profile: user yang login beserta peran dan masa langganannya. SubscriptionExpiresAt hanya terisi untuk owner.
type Profile struct {
	// User: data akun user yang login.
	User User
	// IsOwner: true bila user punya antrian (baris di tabel owners).
	IsOwner bool
	// SubscriptionExpiresAt: akhir masa langganan owner. Nil untuk customer.
	SubscriptionExpiresAt *time.Time
	// AvatarUpdatedAt: nil bila user belum punya foto profil.
	AvatarUpdatedAt *time.Time
	// EmailVerified: true bila email sudah diverifikasi, atau bila verifikasi email dimatikan (tidak ada yang perlu dilakukan user).
	EmailVerified bool
}

// Avatar: foto profil hasil olahan (JPEG).
type Avatar struct {
	// Data: isi berkas gambar yang sudah diolah dan di-encode ulang.
	Data []byte
	// ContentType: tipe MIME berkas, mis. image/jpeg.
	ContentType string
	// UpdatedAt: waktu foto terakhir diganti. Dipakai klien sebagai penanda cache.
	UpdatedAt time.Time
}
