# =============================================================================
# Dockerfile: kontainer payment-service QMS (Go; pgx + stripe-go).
# -----------------------------------------------------------------------------
# Build context : folder payment-service/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Dependensi    : pgx (database) dan stripe-go (klien Stripe, webhook).
# Target        : dev   -> go run dari kode yang di-mount.
#                 build -> tahap antara: mengompilasi binary.
#                 prod  -> binary statis di alpine kecil, user non-root, ada HEALTHCHECK.
#
# Saat ada perubahan, periksa:
#   [SINKRON] PORT default 8093. HEALTHCHECK memakai ${PORT:-8093}, dan upstream $qms_payment di
#             deploy/nginx/server.conf memakai payment-service:8093. Ketiganya harus sama.
#   [PENTING] USER app (UID 10001) dan adduser: jangan dijalankan sebagai root.
#   [PENTING] Kunci Stripe dan JWT_SECRET diisi saat run, tidak dibakar ke image.
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
# Menyalin seluruh folder payment-service ke /src.
COPY . .
# Menjalankan server langsung dari kode sumber.
CMD ["go", "run", "./cmd/payment-service"]

# Tahap build: mengompilasi binary tunggal. Tidak ikut ke image akhir.
FROM base AS build
# Salin seluruh kode sumber untuk dikompilasi.
COPY . .
# CGO_ENABLED=0: binary tanpa C. GOOS=linux: target Linux. -trimpath: menghapus path lokal dari binary.
# -ldflags="-s -w": membuang simbol debug agar binary lebih kecil.
# -o /out/payment-service: hasil binary disimpan di /out/payment-service, lalu disalin ke tahap prod.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/payment-service ./cmd/payment-service

# Tahap prod (target terakhir = default): image kecil berisi binary saja.
FROM alpine:3.20 AS prod
# ca-certificates: sertifikat root untuk HTTPS ke API Stripe.
# tzdata: data zona waktu IANA untuk menulis waktu kedaluwarsa langganan dengan benar.
# adduser -D -u 10001 app: user non-root dengan UID tetap 10001, tanpa password.
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
# Menyalin binary hasil build ke PATH. Hanya file ini yang masuk image akhir.
COPY --from=build /out/payment-service /usr/local/bin/payment-service
# [PENTING] Menjalankan proses sebagai user app (non-root).
USER app
# Pemeriksaan kesehatan: memanggil /health di port PORT (default 8093) setiap 10 detik.
# --timeout=3s: batas satu pemeriksaan. --start-period=5s: kegagalan di 5 detik pertama belum dihitung.
# --retries=6: 6 kegagalan berturut-turut sebelum unhealthy.
# wget -qO- ... || exit 1: gagal (non-zero) bila endpoint tidak menjawab.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 \
    # Perintah yang dijalankan Docker di dalam kontainer untuk memeriksa kesehatan (lanjutan dari HEALTHCHECK di atas).
    CMD wget -qO- http://127.0.0.1:${PORT:-8093}/health || exit 1
# Perintah default saat container dijalankan: binary payment-service.
CMD ["payment-service"]
