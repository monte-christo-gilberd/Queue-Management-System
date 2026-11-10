# =============================================================================
# Dockerfile: kontainer auth-service QMS (Go; gin + sqlx/pgx + golang-jwt).
# -----------------------------------------------------------------------------
# Build context : folder auth-service/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Target        : dev   -> go run dari kode yang di-mount.
#                 build -> tahap antara: mengompilasi binary.
#                 prod  -> binary statis di alpine kecil, user non-root, ada HEALTHCHECK.
#
# Saat ada perubahan, periksa:
#   [SINKRON] PORT default 8094. HEALTHCHECK memakai ${PORT:-8094}, dan upstream $qms_auth di
#             deploy/nginx/server.conf memakai auth-service:8094. Ketiganya harus sama.
#   [PENTING] USER app (UID 10001): jangan dijalankan sebagai root.
#   [PENTING] Kunci privat JWT TIDAK dibakar ke image. Di-mount saat run lewat AUTH_JWT_PRIVATE_KEY_FILE
#             (dibuat scripts/gen-auth-keys.sh). Hanya layanan ini yang boleh memegangnya.
# =============================================================================

# Tahap dasar bersama: Go 1.27 di atas Alpine. Dipakai oleh tahap dev dan build.
FROM golang:1.27-alpine AS base
# Direktori kerja di dalam kontainer.
WORKDIR /src
# Hanya go.mod dan go.sum yang disalin dulu, supaya layer unduhan modul tetap ada di cache saat kode berubah.
COPY go.mod go.sum ./
# Mengunduh semua modul yang dibutuhkan go.mod ke cache modul.
RUN go mod download

# Tahap dev: dipakai docker-compose.yml. Kode di-mount, jadi perubahan langsung terlihat setelah restart.
FROM base AS dev
# Menyalin seluruh folder auth-service ke /src.
COPY . .
# Menjalankan server langsung dari kode sumber.
CMD ["go", "run", "./cmd/auth-service"]

# Tahap build: mengompilasi binary tunggal. Tidak ikut ke image akhir.
FROM base AS build
# Salin seluruh kode sumber untuk dikompilasi.
COPY . .
# CGO_ENABLED=0: binary tanpa C. -trimpath: menghapus path lokal. -ldflags="-s -w": membuang simbol debug.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-service ./cmd/auth-service

# Tahap prod (target terakhir = default): image kecil berisi binary saja.
FROM alpine:3.20 AS prod
# ca-certificates: sertifikat root. adduser -D -u 10001 app: user non-root dengan UID tetap, tanpa password.
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
# Menyalin binary hasil build ke PATH. Hanya file ini yang masuk image akhir.
COPY --from=build /out/auth-service /usr/local/bin/auth-service
# [PENTING] Menjalankan proses sebagai user app (non-root).
USER app
# Pemeriksaan kesehatan: /health di port PORT (default 8094) setiap 10 detik; 6 kegagalan berturut-turut = unhealthy.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 \
    CMD wget -qO- http://127.0.0.1:${PORT:-8094}/health || exit 1
# Perintah default saat container dijalankan: binary auth-service.
CMD ["auth-service"]
