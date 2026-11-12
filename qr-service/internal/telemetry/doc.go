// Package telemetry menyalakan tracing OpenTelemetry qr-service: exporter OTLP gRPC ke collector, sampling head, dan
// propagator W3C. Tracing hanya aktif bila OTEL_EXPORTER_OTLP_ENDPOINT terisi; kosong berarti no-op, jadi aplikasi
// tidak butuh observability untuk start dan tidak pernah menunggu collector.
//
// Isinya:
//   - telemetry.go: Setup, Enabled, sampling (OTEL_TRACES_SAMPLER_ARG, bawaan 10%).
//   - redact.go: pagar privasi span, membuang path mentah, query, header, alamat klien, dan menyamarkan rahasia.
//   - gin.go: Middleware, span server per request (otelgin) dengan nama berupa templat route.
package telemetry
