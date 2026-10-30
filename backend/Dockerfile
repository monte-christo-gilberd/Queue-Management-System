# Kontainer backend (Go, Gin). Build context: folder backend/
# Target:
#   dev  (go run, kode di-mount; restart manual: docker compose restart backend)   docker compose up
#   prod (default, binary statis di alpine kecil)  docker compose up --build
FROM golang:1.27-alpine AS base
WORKDIR /src
# Unduh dependensi dulu agar layer cache tetap berguna saat kode berubah.
COPY go.mod go.sum ./
RUN go mod download

# ---- dev: go run dari kode yang di-mount; ubah kode lalu `docker compose restart backend` ----
FROM base AS dev
COPY . .
ENV PORT=8080
EXPOSE 8080
CMD ["go", "run", "./cmd/server"]

# ---- build ----
FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qms ./cmd/server

# ---- prod (target terakhir = default) ----
FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

COPY --from=build /out/qms /usr/local/bin/qms

# Konfigurasi lewat environment saat run (tidak dibakar ke image):
#   DATABASE_URL, JWT_SECRET, ALLOWED_ORIGINS, PUBLIC_APP_URL, PUBLIC_QR_URL,
#   QR_SERVICE_URL, QR_SERVICE_TOKEN, TRUSTED_PROXIES (opsional), PORT (default 8080).
ENV PORT=8080
USER app
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:${PORT}/health || exit 1

ENTRYPOINT ["/usr/local/bin/qms"]
