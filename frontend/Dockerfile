# Frontend web (Expo). Build context: folder frontend/
# Target:
#   dev  : Metro dev server dengan hot reload (port 80), kode di-mount
#   prod : expo export, disajikan nginx (default)
FROM node:22-alpine AS base
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# ---- dev ----
FROM base AS dev
COPY . .
ENV EXPO_NO_TELEMETRY=1 \
    EXPO_PUBLIC_API_URL=http://localhost:8080
# Port 80 agar sama dengan target prod: nginx reverse proxy meneruskan `/` ke frontend:80.
EXPOSE 80
CMD ["npx", "expo", "start", "--web", "--host", "lan", "--port", "80"]

# ---- build ----
FROM base AS build
COPY . .
# EXPO_PUBLIC_* dibakar ke bundle saat build (publik, bukan rahasia). Wajib diisi dengan origin nginx,
# mis. --build-arg EXPO_PUBLIC_API_URL=https://qms.contoh.ac.id. Kosong = frontend memanggil host:8080 langsung.
ARG EXPO_PUBLIC_API_URL
ENV EXPO_PUBLIC_API_URL=${EXPO_PUBLIC_API_URL}
RUN npx expo export --platform web --clear

# ---- prod (target terakhir = default): nginx menyajikan hasil build ----
FROM nginx:1.27-alpine AS prod
COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
