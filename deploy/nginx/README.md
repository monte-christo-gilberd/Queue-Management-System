# Konfigurasi nginx (QMS)

Mengikuti dua template di `template-nginx/`:

| Berkas | Template | Fungsi |
|---|---|---|
| `deploy/nginx/nginx.conf` | `nginx-reverse-proxy.conf.example` | Reverse proxy, satu-satunya pintu masuk (port 80) |
| `deploy/nginx/frontend-upgrade.dev.conf` / `.prod.conf` | - | Header WebSocket untuk HMR Metro. Hanya dev; di prod berkasnya kosong (build arg `FRONTEND_UPGRADE`) |
| `frontend/nginx.conf` | `nginx-frontend.conf.example` | nginx di kontainer frontend, menyajikan hasil `expo export` |

Nama host di config adalah nama service di `docker-compose.yml`: `backend:8080`, `qr-service:8090`, `frontend:80`.

## Routing reverse proxy

| Path | Tujuan | Catatan |
|---|---|---|
| `/api/v1/queues/:id/event` | `backend` | SSE: tanpa buffer, timeout 86400s |
| `/api/v1/users/login`, `/register` | `backend` | Rate limit 10 r/s per IP (burst 20), lewat batas dibalas 429 |
| `/api/v1/join`, `/join/preview` | `backend` | Rate limit 10 r/s per IP (burst 20) |
| `/api/` | `backend` | Timeout 60s |
| `/internal/` | - | Selalu 404 (endpoint internal qr-service) |
| `/` | `frontend:80` | Aplikasi web |

Header proxy umum (`Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`) diatur sekali di level `server`. Header keamanan dasar dan `client_max_body_size 1m` ikut di sana.

## nginx frontend

Satu HTML per rute dari Expo (mis. `dashboard.html`), jadi `try_files $uri $uri.html $uri/ /index.html`. Aset statis di-cache setahun, HTML tidak di-cache.

## Menjalankan

```bash
cp .env.example .env        # isi secret dan QMS_PUBLIC_ORIGIN
docker compose up --build   # nginx terbuka di ${QMS_HTTP_PORT:-8080}
```

Hanya ada satu `docker-compose.yml` (mode development, hot reload). Hanya nginx yang membuka port; `db`, `backend`, `qr-service`, dan `frontend` hanya terjangkau lewat jaringan Docker. Psql: `docker compose exec db psql -U qms -d qms_db`.

`QMS_PUBLIC_ORIGIN` harus sama dengan alamat yang dibuka browser dan HP (dipakai backend dan qr-service untuk CORS dan `join_url`). Frontend memanggil API di origin halaman, jadi tidak ada yang dibakar ke bundle.

`backend` memakai `TRUSTED_PROXIES` (subnet jaringan compose) agar IP klien dari `X-Forwarded-For` dipercaya hanya bila datang dari nginx.

## Belum ada

- HSTS dan sertifikat produksi. Untuk uji dari HP pakai tunnel (bagian di bawah).
- Mailpit (`/mail/`) dan `notify-service`. Tambahkan saat layanannya ada.

## HTTPS publik tanpa konfigurasi di HP (tunnel)

HP tidak perlu memasang apa pun: Cloudflare quick tunnel memberi URL `https://*.trycloudflare.com` dengan sertifikat yang dipercaya publik, jadi kamera pemindai QR langsung jalan.

```bash
# pasang cloudflared: https://github.com/cloudflare/cloudflared/releases
make tunnel   # buka tunnel, jalankan stack, cetak URL
make down     # hentikan
```

- URL acak dan berganti tiap sesi. Skrip memberikannya sebagai `QMS_PUBLIC_ORIGIN` ke backend dan qr-service (yang membacanya saat start). Frontend memakai `EXPO_PUBLIC_API_URL=origin` (origin halaman), jadi tidak perlu build ulang saat URL berganti.
- Butuh internet. Jangan dipakai untuk data nyata: siapa pun yang tahu URL-nya bisa mengakses aplikasi.
- Jaringan yang memblokir QUIC ditangani dengan `--protocol http2`.

## Uji cepat

- `/` menampilkan halaman login, dan `/dashboard` tidak 404.
- `/api/v1/queues` tanpa token membalas 401 (502 berarti backend mati).
- `/internal/v1/join-token` membalas 404.
- Banyak request cepat ke `/api/v1/users/login` akhirnya dibalas 429.
- Owner menekan Next dan layar customer berubah tanpa refresh (SSE tidak tertahan buffer).
