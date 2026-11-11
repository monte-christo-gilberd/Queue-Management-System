package service

import (
	"context"

	"qms/notify-service/internal/model"
)

// NotifyServiceInterface: kontrak service notifikasi yang dipakai runner. Dipenuhi *NotifyService.
type NotifyServiceInterface interface {
	// Plan: menghitung penerima dan isi pesan untuk satu event, tanpa mengirim apa pun.
	Plan(
		ctx context.Context,
		e model.Event,
	) ([]Delivery, error)
	// Handle: merencanakan lalu mengirim semua push untuk satu event.
	Handle(
		ctx context.Context,
		e model.Event,
	) error
}
