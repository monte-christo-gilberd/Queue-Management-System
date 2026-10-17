# Build frontend web (Expo static export). Build context: folder frontend/
# Hasil build disalin ke image kecil berisi folder /dist saja. Image ini tidak dijalankan sebagai server:
# isinya dipakai untuk mengisi volume yang dibaca kontainer nginx.
FROM node:22-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# EXPO_PUBLIC_* dibakar ke bundle saat build (publik, bukan rahasia). Wajib diisi dengan origin nginx,
# mis. --build-arg EXPO_PUBLIC_API_URL=https://qms.contoh.ac.id. Kosong = frontend memanggil host:8080 langsung.
ARG EXPO_PUBLIC_API_URL
ENV EXPO_PUBLIC_API_URL=${EXPO_PUBLIC_API_URL}
RUN npx expo export --platform web --clear

FROM busybox:1.36
COPY --from=build /app/dist /dist
# Pemakaian: docker run --rm -v qms_frontend:/target <image> sh -c 'cp -r /dist/. /target/'
