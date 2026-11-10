# =============================================================================
# Dockerfile: kontainer backend QMS (Go, Gin).
# -----------------------------------------------------------------------------
# Build context : folder backend/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Target        : dev   -> go run dari kode yang di-mount. Ubah kode lalu
#                          `docker compose restart backend` (tidak ada hot reload).
#                 build -> tahap antara: mengompilasi binary. Tidak dijalankan langsung.
#                 prod  -> binary statis di alpine kecil, berjalan sebagai user non-root.
# Dijalankan    : `docker compose up` (dev), `docker compose up --build` (prod).
#
# Konfigurasi lewat environment saat run, TIDAK dibakar ke image:
#   DATABASE_URL, JWT_PUBLIC_KEY_FILE (di-mount), ALLOWED_ORIGINS, PUBLIC_APP_URL, PUBLIC_QR_URL,
#   QR_SERVICE_URL, QR_SERVICE_TOKEN, TRUSTED_PROXIES (opsional), PORT (default 8080).
#
# Saat ada perubahan, periksa:
#   [SINKRON] EXPOSE 8080, ENV PORT=8080, dan HEALTHCHECK harus sama dengan PORT yang dipakai compose
#             dan upstream $qms_backend di deploy/nginx/server.conf (backend:8080).
#   [PENTING] USER app dan adduser -u 10001: jangan dijalankan sebagai root.
# =============================================================================

# Tahap dasar bersama: image Go lengkap dengan toolchain 1.27 di atas Alpine.
# AS base memberi nama tahap ini agar tahap lain bisa memakainya (FROM base).
FROM golang:1.27-alpine AS base
# Direktori kerja di dalam kontainer. Semua perintah berikutnya berjalan di sini.
WORKDIR /src
# Unduh dependensi dulu agar layer cache tetap berguna saat kode berubah.
# Hanya go.mod dan go.sum yang disalin di tahap ini, jadi layer ini tidak berubah setiap kali kode diedit.
COPY go.mod go.sum ./
# Mengunduh semua modul yang dibutuhkan go.mod ke cache modul. Dijalankan sekali di layer ini.
RUN go mod download

# ---- dev: go run dari kode yang di-mount; ubah kode lalu `docker compose restart backend` ----
# Tahap dev: dipakai oleh docker-compose.yml. Kode di-mount, jadi perubahan langsung terlihat.
FROM base AS dev
# Menyalin seluruh folder backend ke /src (sebagian besar dibatasi oleh Dockerfile.dockerignore).
COPY . .
# Port yang didengarkan server. Di compose bisa diubah lewat PORT.
ENV PORT=8080
# Dokumentasi: kontainer mendengarkan port 8080. Pemetaan ke host dilakukan di docker-compose.
EXPOSE 8080
# Menjalankan server langsung dari kode sumber. Lambat start, tetapi tidak perlu build ulang.
CMD ["go", "run", "./cmd/server"]

# ---- build ----
# Tahap build: mengompilasi binary tunggal. Tidak ikut ke image akhir.
FROM base AS build
# Salin seluruh kode sumber untuk dikompilasi.
COPY . .
# CGO_ENABLED=0: binary tanpa C, bisa jalan di alpine tanpa libc.
# GOOS=linux: target sistem operasi Linux.
# -trimpath: menghapus path lokal dari binary (lebih reproducible, tidak membocorkan path build).
# -ldflags="-s -w": membuang tabel simbol dan DWARF, memperkecil binary.
# -o /out/qms: hasil binary disimpan sebagai /out/qms. Entrypoint di tahap prod memakai nama ini.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qms ./cmd/server

# ---- prod (target terakhir = default) ----
# Tahap prod: image kecil yang hanya berisi binary dan runtime minimal. Ini target default.
FROM alpine:3.20 AS prod
# ca-certificates: sertifikat root untuk HTTPS (mis. panggilan ke layanan luar).
# tzdata: data zona waktu IANA, agar waktu bisa ditampilkan dengan zona yang benar.
# adduser -D -u 10001 app: membuat user non-root "app" dengan UID tetap 10001 tanpa password.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

# Menyalin binary hasil build dari tahap build. Hanya file ini yang masuk image akhir.
COPY --from=build /out/qms /usr/local/bin/qms

# Konfigurasi lewat environment saat run (tidak dibakar ke image):
#   DATABASE_URL, JWT_PUBLIC_KEY_FILE (di-mount), ALLOWED_ORIGINS, PUBLIC_APP_URL, PUBLIC_QR_URL,
#   QR_SERVICE_URL, QR_SERVICE_TOKEN, TRUSTED_PROXIES (opsional), PORT (default 8080).
# Port default server. Harus sama dengan upstream di nginx (backend:8080).
ENV PORT=8080
# [PENTING] Menjalankan proses sebagai user app (non-root). Jika proses diretas, dampaknya terbatas.
USER app
# Dokumentasi: port yang didengarkan. Pemetaan ke host dilakukan di docker-compose.
EXPOSE 8080

# Pemeriksaan kesehatan: Docker memanggil /health setiap 15 detik. 3 kegagalan berturut-turut = unhealthy.
# --timeout=3s: batas satu pemeriksaan. --start-period=10s: kegagalan di 10 detik pertama belum dihitung.
# wget -qO- ... || exit 1: gagal (non-zero) bila endpoint tidak menjawab.
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    # Perintah yang dijalankan Docker di dalam kontainer untuk memeriksa kesehatan (lanjutan dari HEALTHCHECK di atas).
    CMD wget -qO- http://127.0.0.1:${PORT}/health || exit 1

# Titik masuk kontainer: menjalankan binary qms. Argumen dari `docker run` ditambahkan di belakangnya.
ENTRYPOINT ["/usr/local/bin/qms"]
