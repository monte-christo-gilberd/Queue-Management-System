package service

import (
	"context"
	"time"
)

// subscriptionStore adalah kontrak repository langganan (dipenuhi repository.SubscriptionRepository).
type subscriptionStore interface {
	// Extend menambah 30 hari ke langganan userID dan mengembalikan tanggal berakhir yang baru.
	Extend(ctx context.Context, userID int64) (time.Time, error)
}

// SubscriptionService mengurus penambahan masa langganan owner tanpa pembayaran (mock: tombol di layar Subscription).
type SubscriptionService struct {
	// store menulis perpanjangan ke database.
	store subscriptionStore
}

// NewSubscriptionService membuat SubscriptionService dengan repository yang diberikan.
func NewSubscriptionService(
	store subscriptionStore,
) *SubscriptionService {
	return &SubscriptionService{
		store: store,
	}
}

// Extend menambah 30 hari ke langganan requesterID dan mengembalikan tanggal berakhir yang baru.
func (s *SubscriptionService) Extend(
	ctx context.Context,
	requesterID int64,
) (time.Time, error) {
	return s.store.Extend(ctx, requesterID)
}
