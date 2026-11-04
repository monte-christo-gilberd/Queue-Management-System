package model

import "time"

// Profile: user yang login beserta peran dan masa langganannya. SubscriptionExpiresAt hanya terisi untuk owner.
type Profile struct {
	User                  User
	IsOwner               bool
	SubscriptionExpiresAt *time.Time
}
