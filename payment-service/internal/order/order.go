// Package order berisi tipe dan error pesanan pembayaran yang dipakai bersama oleh checkout, webhook, dan store.
package order

import "errors"

// Status pesanan di tabel payment_orders.
const (
	StatusPending = "pending"
	StatusPaid    = "paid"
	StatusExpired = "expired"
	StatusFailed  = "failed"
)

// PlanMonthly: satu-satunya paket yang saat ini bisa dibayar (tambah 30 hari langganan).
const PlanMonthly = "monthly"

// ErrUnknown: reference_id webhook tidak ada di payment_orders.
var ErrUnknown = errors.New("order: reference_id tidak dikenal")

// ErrAmountMismatch: nominal di webhook berbeda dengan nominal pesanan. Pembayaran tidak diterapkan.
var ErrAmountMismatch = errors.New("order: nominal webhook tidak sesuai pesanan")

// ErrDuplicate: reference_id sudah pernah dibuat untuk checkout.
var ErrDuplicate = errors.New("order: reference_id sudah ada")

// Pending: pesanan yang dibuat saat checkout, sebelum customer membayar.
type Pending struct {
	// ReferenceID: ID pesanan dari backend utama. Unik.
	ReferenceID string
	// OwnerID: owner yang langganannya diperpanjang bila pembayaran lunas.
	OwnerID int64
	// Amount: nominal dalam rupiah yang harus cocok dengan webhook.
	Amount int64
	// PlanType: paket yang dibeli (saat ini hanya PlanMonthly).
	PlanType string
}
