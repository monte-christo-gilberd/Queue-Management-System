// Package service berisi logika bisnis qr-service: aturan pembuatan dan verifikasi token join, serta aturan
// render QR. Handler HTTP memanggil service. Service tidak mengenal HTTP dan hanya mengembalikan apperror.
//
// Service ini stateless dan tidak mengakses database, file, atau jaringan, jadi tidak ada lapisan repository.
//
// Isinya:
//   - service_interface.go: TokenServiceInterface dan RenderServiceInterface, kontrak yang dipenuhi service.
//   - token_service.go: TokenService (NewTokenService, IssueToken, Issue, VerifyToken) dan aturan TTL.
//   - token_signer.go: pembantu tanda tangan HMAC-SHA256 dan id acak (sign, randomID).
//   - render_service.go: RenderService (NewRenderService, Render), batas ukuran, tingkat koreksi, dan warna.
//
// Format token: "v1.<payload base64url>.<tanda tangan base64url>". Package ini tidak tahu apakah token masih
// cocok dengan database; backend yang mencocokkan token statis dengan queue_settings.
package service
