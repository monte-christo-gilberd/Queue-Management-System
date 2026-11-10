// Package httpapi berisi lapisan HTTP qr-service: router Gin, handler endpoint, middleware, dan server.
//
// Endpoint dibagi dua:
//   - Publik: /health dan /ready untuk probe container.
//   - Internal: /internal/v1/* untuk backend. Setiap request wajib membawa header X-Service-Token dan
//     tidak diproksikan nginx.
//
// Isinya:
//   - router.go: Config, NewRouter (memasang middleware dan rute), dan NewServer (timeout HTTP).
//   - handlers.go: Handlers dan endpoint-nya: Health, Ready, IssueToken, VerifyToken, RenderGeneric.
//   - middleware.go: ServiceToken, BodyLimit, RequestID, Logger, dan NoStore.
//
// Package ini hanya menerjemahkan HTTP ke pemanggilan joinlink dan qr. Logika token ada di joinlink.
package httpapi
