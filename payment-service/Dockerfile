# =============================================================================
# Dockerfile: kontainer payment-service QMS (Go). Membuat pembayaran BorderPay dan menerima webhook-nya.
# -----------------------------------------------------------------------------
# Build context : folder payment-service/ (docker-compose.yml: build.context).
# Target        : dev   -> go run dari kode yang di-mount. Ubah kode lalu `docker compose restart payment-service`.
#                 build -> tahap antara: mengompilasi binary.
#                 prod  -> binary statis di alpine kecil, user non-root, ada HEALTHCHECK.
# Akses database: hanya lewat role payment_rw (PAYMENT_DATABASE_URL). Tidak membaca tabel lain selain yang di-GRANT.
#
# Saat ada perubahan, periksa:
#   [SINKRON] PORT 8095. nginx (deploy/nginx/server.conf, $qms_payment) memakai port yang sama.
#   [PENTING] BORDERPAY_API_KEY, BORDERPAY_WEBHOOK_TOKEN, PAYMENT_SERVICE_TOKEN, PAYMENT_DATABASE_URL diisi saat run.
#   [PENTING] USER app (UID 10001): jangan dijalankan sebagai root.
# =============================================================================

FROM golang:1.27-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# ---- dev: go run; ubah kode lalu `docker compose restart payment-service` ----
FROM base AS dev
COPY . .
ENV PORT=8095
EXPOSE 8095
CMD ["go", "run", "./cmd/payment-service"]

# ---- build ----
FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/payment-service ./cmd/payment-service

# ---- prod (target terakhir = default) ----
FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 app
COPY --from=build /out/payment-service /usr/local/bin/payment-service
USER app
EXPOSE 8095
# Port 8095 ditulis langsung karena tahap prod tidak mengatur ENV PORT.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8095/health || exit 1
ENTRYPOINT ["/usr/local/bin/payment-service"]
