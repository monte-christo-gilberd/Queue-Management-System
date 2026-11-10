# =============================================================================
# Dockerfile: kontainer notify-service QMS (Go). Mengirim push notifikasi ke browser.
# -----------------------------------------------------------------------------
# Build context : folder notify-service/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Target        : dev   -> go run dari kode yang di-mount.
#                 build -> tahap antara: mengompilasi binary.
#                 prod  -> binary statis di alpine kecil, user non-root.
#                 Catatan: service ini punya endpoint /health di port 8091 (cmd/notify-service/main.go),
#                 tetapi Dockerfile belum memasang HEALTHCHECK untuknya. Pasang bila ingin Docker memantaunya.
#
# Saat ada perubahan, periksa:
#   [PENTING] USER app (UID 10001) dan adduser: jangan dijalankan sebagai root.
#   [PENTING] Konfigurasi VAPID dan DATABASE_URL harus diisi saat run (lihat config.Load).
#             Tanpa itu, service gagal start dengan pesan yang jelas.
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
# Menyalin seluruh folder notify-service ke /src.
COPY . .
# Menjalankan server langsung dari kode sumber.
CMD ["go", "run", "./cmd/notify-service"]

# Tahap build: mengompilasi binary tunggal. Tidak ikut ke image akhir.
FROM base AS build
# Salin seluruh kode sumber untuk dikompilasi.
COPY . .
# CGO_ENABLED=0: binary tanpa C. GOOS=linux: target Linux. -trimpath: menghapus path lokal dari binary.
# -ldflags="-s -w": membuang simbol debug agar binary lebih kecil.
# -o /out/notify-service: hasil binary disimpan di /out/notify-service, lalu disalin ke tahap prod.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notify-service ./cmd/notify-service

# Tahap prod (target terakhir = default): image kecil berisi binary saja.
FROM alpine:3.20 AS prod
# ca-certificates: sertifikat root untuk HTTPS saat mengirim push ke server push browser.
# adduser -D -u 10001 app: user non-root dengan UID tetap 10001, tanpa password.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
# Menyalin binary hasil build ke PATH. Hanya file ini yang masuk image akhir.
COPY --from=build /out/notify-service /usr/local/bin/notify-service
# [PENTING] Menjalankan proses sebagai user app (non-root).
USER app
# Perintah default saat container dijalankan: binary notify-service.
CMD ["notify-service"]
