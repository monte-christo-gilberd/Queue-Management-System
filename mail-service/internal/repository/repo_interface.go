package repository

import (
	"context"
	"time"

	"qms/mail-service/internal/model"
)

// OutboxRepositoryInterface: kontrak akses tabel notification_outbox. Dipenuhi *OutboxRepository, dan dipakai
// tes untuk membuat tiruan dalam memori.
type OutboxRepositoryInterface interface {
	// Claim mengklaim paling banyak limit baris yang jatuh tempo, menaikkan attempts, dan menyewa baris
	// sebesar lease supaya worker lain tidak mengambilnya bersamaan.
	Claim(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]model.OutboxItem, error)
	// MarkSent menandai baris sudah terkirim.
	MarkSent(
		ctx context.Context,
		id int64,
	) error
	// MarkRetry menjadwalkan percobaan ulang setelah delay dan menyimpan penyebab galat terakhir.
	MarkRetry(
		ctx context.Context,
		id int64,
		cause string,
		delay time.Duration,
	) error
	// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya.
	MarkFailed(
		ctx context.Context,
		id int64,
		cause string,
	) error
}

// Pastikan implementasi PostgreSQL memenuhi kontrak.
var _ OutboxRepositoryInterface = (*OutboxRepository)(nil)
