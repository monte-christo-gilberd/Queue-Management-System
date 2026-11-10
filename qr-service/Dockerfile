# =============================================================================
# Dockerfile: kontainer qr-service QMS (Go). Membuat dan memverifikasi token join QR.
# -----------------------------------------------------------------------------
# Build context : folder qr-service/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Target        : dev   -> go run. Ubah kode lalu `docker compose restart qr-service`.
#                 build -> tahap antara: mengompilasi binary.
#                 prod  -> binary statis di alpine kecil, user non-root, ada HEALTHCHECK.
# Tidak punya akses database. Service ini stateless.
#
# Saat ada perubahan, periksa:
#   [SINKRON] PORT 8090. Backend memanggilnya lewat QR_SERVICE_URL dengan port yang sama.
#   [PENTING] JOIN_LINK_SECRET dan QR_SERVICE_TOKEN diisi saat run, tidak dibakar ke image.
#   [PENTING] USER app (UID 10001) dan adduser: jangan dijalankan sebagai root.
# =============================================================================

# Tahap dasar bersama: Go 1.27 di atas Alpine. Dipakai oleh tahap dev dan build.
FROM golang:1.27-alpine AS base
# Direktori kerja di dalam kontainer.
WORKDIR /src
# Hanya go.mod dan go.sum yang disalin dulu, supaya layer unduhan modul tetap ada di cache saat kode berubah.
COPY go.mod go.sum ./
# Mengunduh semua modul yang dibutuhkan go.mod ke cache modul.
RUN go mod download

# ---- dev: go run; ubah kode lalu `docker compose restart qr-service` ----
# Tahap dev: dipakai docker-compose.yml. Kode di-mount, jadi perubahan langsung terlihat setelah restart.
FROM base AS dev
# Menyalin seluruh folder qr-service ke /src.
COPY . .
# Port yang didengarkan server. Nilai ini dibaca config.Load, bisa diubah lewat compose.
ENV PORT=8090
# Dokumentasi: kontainer mendengarkan port 8090. Pemetaan ke host dilakukan di docker-compose.
EXPOSE 8090
# Menjalankan server langsung dari kode sumber.
CMD ["go", "run", "./cmd/qr-service"]

# ---- build ----
# Tahap build: mengompilasi binary tunggal. Tidak ikut ke image akhir.
FROM base AS build
# Salin seluruh kode sumber untuk dikompilasi.
COPY . .
# CGO_ENABLED=0: binary tanpa C. GOOS=linux: target Linux. -trimpath: menghapus path lokal dari binary.
# -ldflags="-s -w": membuang simbol debug agar binary lebih kecil.
# Dipecah ke dua baris dengan "\" agar mudah dibaca; hasilnya sama dengan satu baris.
# -o /out/qr-service: hasil binary disimpan di /out/qr-service, lalu disalin ke tahap prod.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/qr-service ./cmd/qr-service

# ---- prod (target terakhir = default) ----
# Tahap prod: image kecil berisi binary saja. Ini target default.
FROM alpine:3.20 AS prod

# ca-certificates: sertifikat root untuk koneksi HTTPS. Diperlukan bila service memanggil layanan luar.
# adduser -D -u 10001 app: user non-root dengan UID tetap 10001, tanpa password.
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 app

# Menyalin binary hasil build ke PATH. Hanya file ini yang masuk image akhir.
COPY --from=build /out/qr-service /usr/local/bin/qr-service

# [PENTING] Menjalankan proses sebagai user app (non-root).
USER app
# Dokumentasi: port yang didengarkan. Pemetaan ke host dilakukan di docker-compose.
EXPOSE 8090

# Pemeriksaan kesehatan: memanggil /health di port 8090 setiap 30 detik. 3 kegagalan berturut-turut = unhealthy.
# --timeout=3s: batas satu pemeriksaan. --start-period=5s: kegagalan di 5 detik pertama belum dihitung.
# Port 8090 ditulis langsung, tidak memakai ${PORT}, karena tahap prod tidak mengatur ENV PORT (hanya tahap dev).
# wget -qO- ... || exit 1: gagal (non-zero) bila endpoint tidak menjawab.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    # Perintah yang dijalankan Docker di dalam kontainer untuk memeriksa kesehatan (lanjutan dari HEALTHCHECK di atas).
    CMD wget -qO- http://127.0.0.1:8090/health || exit 1

# Titik masuk kontainer: menjalankan binary qr-service. Argumen dari `docker run` ditambahkan di belakangnya.
ENTRYPOINT ["/usr/local/bin/qr-service"]
