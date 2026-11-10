# =============================================================================
# Dockerfile: kontainer frontend QMS (Expo, web).
# -----------------------------------------------------------------------------
# Build context : folder frontend/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml (dev, target dev) dan docker-compose.prod.yml
#                 (prod, target default = prod).
# Target        : dev   -> Metro dev server dengan hot reload di port 80. Kode di-mount.
#                 build -> tahap antara: `expo export` menghasilkan folder dist/ berisi HTML dan aset statis.
#                 prod  -> nginx yang menyajikan dist/ (default).
#
# Saat ada perubahan, periksa:
#   [SINKRON] EXPOSE 80 harus sama dengan upstream $qms_frontend di deploy/nginx/server.conf (frontend:80).
#   [PENTING] EXPO_PUBLIC_API_URL dibakar ke bundle JavaScript saat build. Nilainya publik (bukan rahasia),
#             tetapi harus sama dengan origin yang dibuka browser. Lihat ARG di tahap build.
#   [SINKRON] COPY nginx.conf di tahap prod memakai frontend/nginx.conf (bukan deploy/nginx/).
# =============================================================================

# Tahap dasar bersama: Node 22 di atas Alpine. Dipakai oleh tahap dev dan build.
FROM node:22-alpine AS base
# Direktori kerja di dalam kontainer.
WORKDIR /app
# Hanya package.json dan package-lock.json yang disalin dulu, supaya layer npm ci tetap ada di cache saat kode berubah.
COPY package.json package-lock.json ./
# npm ci: memasang dependensi persis sesuai package-lock.json. Lebih cepat dan konsisten dibanding npm install.
RUN npm ci

# ---- dev ----
# Tahap dev: dipakai docker-compose.yml. Metro berjalan dengan hot reload.
FROM base AS dev
# Menyalin seluruh folder frontend ke /app (sebagian dibatasi oleh .dockerignore).
COPY . .
# EXPO_NO_TELEMETRY=1: mematikan pengiriman telemetri Expo CLI.
# EXPO_PUBLIC_API_URL: origin API untuk mode dev. localhost:8080 adalah port nginx dev di host.
ENV EXPO_NO_TELEMETRY=1 \
    EXPO_PUBLIC_API_URL=http://localhost:8080
# Port 80 agar sama dengan target prod: nginx reverse proxy meneruskan `/` ke frontend:80.
EXPOSE 80
# Menjalankan Metro sebagai server web. --host lan: bisa diakses dari perangkat lain di jaringan. --port 80: sama dengan prod.
CMD ["npx", "expo", "start", "--web", "--host", "lan", "--port", "80"]

# ---- build ----
# Tahap build: membuat bundle statis. Tidak ikut ke image akhir kecuali folder dist/-nya.
FROM base AS build
# Salin seluruh kode sumber untuk di-export.
COPY . .
# EXPO_PUBLIC_* dibakar ke bundle saat build (publik, bukan rahasia). Wajib diisi dengan origin nginx,
# mis. --build-arg EXPO_PUBLIC_API_URL=https://qms.contoh.ac.id. Kosong = frontend memanggil host:8080 langsung.
# ARG: nilai ini hanya ada saat build, dan diambil dari --build-arg.
ARG EXPO_PUBLIC_API_URL
# Meneruskan ARG ke ENV agar `expo export` membacanya dan membakarnya ke bundle.
ENV EXPO_PUBLIC_API_URL=${EXPO_PUBLIC_API_URL}
# Membuat bundle web statis ke folder dist/. --clear: membersihkan cache Metro agar hasilnya bersih.
RUN npx expo export --platform web --clear

# ---- prod (target terakhir = default): nginx menyajikan hasil build ----
# Tahap prod: nginx ringan yang hanya menyajikan file statis. Ini target default.
FROM nginx:1.27-alpine AS prod
# Konfigurasi nginx khusus frontend: try_files untuk rute Expo dan cache aset. Berasal dari frontend/nginx.conf.
COPY nginx.conf /etc/nginx/nginx.conf
# Menyalin hasil export dari tahap build ke folder web nginx. Hanya folder ini yang masuk image akhir.
COPY --from=build /app/dist /usr/share/nginx/html
# Dokumentasi: nginx mendengarkan port 80 di dalam kontainer. Dipetakan oleh nginx reverse proxy.
EXPOSE 80
