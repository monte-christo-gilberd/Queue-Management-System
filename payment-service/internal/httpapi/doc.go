// Package httpapi berisi API HTTP payment-service. Kontrak respons sama dengan backend QMS: sukses
// {"data": ...}, gagal {"error": {"code", "message"}}.
//
// Endpoint:
//   - GET /health: status layanan dan database.
//   - POST /api/v1/payments/checkout: membuka sesi pembayaran untuk user yang login (JWT dari backend).
//   - POST /api/v1/payments/webhooks/stripe: menerima webhook Stripe yang sudah diverifikasi tanda tangannya.
//
// Isinya:
//   - httpapi.go: API (New, Handler, health, checkout, webhook, authenticate) dan interface Billing.
//   - json.go: writeJSON dan writeError, pembantu untuk bentuk respons.
//
// Package ini tidak tahu cara memperpanjang langganan. Itu urusan package billing.
package httpapi
