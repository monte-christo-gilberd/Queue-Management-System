package dto

// PushSubscribeRequest: body POST /push/subscribe (bentuk PushSubscription.toJSON() di browser).
type PushSubscribeRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh" binding:"required"`
		Auth   string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
}

type PushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
}
