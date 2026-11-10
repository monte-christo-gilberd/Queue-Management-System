// Package billing berisi logika pembayaran payment-service. Satu-satunya kemampuannya adalah memperpanjang langganan
// SATU BULAN setelah Stripe mengonfirmasi pembayaran. Pembayaran pertama sebuah user sekaligus menjadikannya owner
// (membuat baris owners). Tidak ada paket lain dan tidak ada penagihan berulang.
//
// Sumber kebenaran adalah webhook Stripe, bukan redirect browser. Webhook yang sama boleh datang berkali-kali,
// dan pembayaran hanya diproses sekali.
//
// Isinya:
//   - billing.go: Service (New, StartCheckout, HandleWebhook, Health), kontrak Provider dan Store, tipe Event,
//     Session, Credit, serta simulator pembayaran palsu untuk dev (WithSimulator).
//   - stripe.go: StripeProvider, implementasi Provider di atas klien stripe-go.
//   - store.go: PGStore, implementasi Store di atas PostgreSQL dengan role payment_rw.
//   - billing.go juga memuat konversi satuan mata uang (MinorFactor, majorUnits).
package billing
