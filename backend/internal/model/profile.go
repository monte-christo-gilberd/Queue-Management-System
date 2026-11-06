package model

import "time"

// Profile: user yang login beserta peran dan masa langganannya. SubscriptionExpiresAt hanya terisi untuk owner.
type Profile struct {
	User                  User
	IsOwner               bool
	SubscriptionExpiresAt *time.Time
	// AvatarUpdatedAt: nil bila user belum punya foto profil.
	AvatarUpdatedAt *time.Time
	// EmailVerified: true bila email sudah diverifikasi, atau bila verifikasi email dimatikan (tidak ada yang perlu dilakukan user).
	EmailVerified bool
}

// Avatar: foto profil hasil olahan (JPEG).
type Avatar struct {
	Data        []byte
	ContentType string
	UpdatedAt   time.Time
}
