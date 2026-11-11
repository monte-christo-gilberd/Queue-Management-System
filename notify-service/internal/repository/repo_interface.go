package repository

import (
	"context"

	"qms/notify-service/internal/model"
)

// NotifyRepositoryInterface: kontrak akses data notifikasi. Dipenuhi *NotifyRepository.
type NotifyRepositoryInterface interface {
	// Ticket: data tiket beserta antrian dan nama counter. Galat bila tiket tidak ada.
	Ticket(
		ctx context.Context,
		ticketID int64,
	) (model.Ticket, error)
	// PositionOf: posisi tiket di urutan tunggu (1 = terdepan).
	PositionOf(
		ctx context.Context,
		ticketID int64,
	) (int, error)
	// FirstWaiting: tiket waiting terdepan di antrian; nil bila tidak ada.
	FirstWaiting(
		ctx context.Context,
		queueID int64,
	) (*model.Ticket, error)
	// Subscriptions: semua langganan push milik user.
	Subscriptions(
		ctx context.Context,
		userID int64,
	) ([]model.Subscription, error)
	// DeleteSubscription: menghapus langganan yang sudah tidak berlaku.
	DeleteSubscription(
		ctx context.Context,
		endpoint string,
	) error
}
