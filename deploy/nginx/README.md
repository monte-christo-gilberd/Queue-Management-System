# Konfigurasi nginx (QMS)

Folder ini berisi konfigurasi reverse proxy: satu-satunya pintu masuk ke seluruh stack. Setiap berkas punya kartu di
kepalanya yang menjelaskan fungsi dan apa yang harus diperiksa bila domain atau topologi berubah. Setiap direktif
diberi komentar sendiri, dan titik yang perlu disesuaikan ditandai dengan penanda yang bisa dicari:

```bash
grep -rn "\[GANTI" deploy/nginx
```

| Penanda | Arti |
|---|---|
| `[GANTI-DOMAIN]` | Nilai terkait domain atau host. Ganti saat pindah domain. |
| `[GANTI-JARINGAN]` | Harus sama dengan `docker-compose*.yml` (nama service, port, subnet). |
| `[SESUAIKAN]` | Nilai tuning (rate limit, burst, batas ukuran, timeout). Boleh diubah sesuai beban. |
| `[SINKRON]` | Duplikat di berkas lain. Ubah di kedua tempat. |
| `[PENTING]` | Keamanan atau kontrak. Jangan diubah tanpa memahami akibatnya. |

## Berkas

| Berkas | Fungsi |
|---|---|
| `nginx.conf` | Konfigurasi **dev** (`docker-compose.yml`): http{}, zona rate limit, format log, `server_name localhost`. Jadi `/etc/nginx/nginx.conf` pada image bawaan. |
| `nginx.prod.conf` | Konfigurasi **prod/demo** di belakang Cloudflare Tunnel: sama dengan dev, ditambah `set_real_ip_from` dan `real_ip_header CF-Connecting-IP`, serta `server_tokens off`. |
| `server.conf` | Isi `server{}` bersama (routing, rate limit per rute, header, pembayaran). Di-include oleh kedua konfigurasi di atas. |
| `frontend-upgrade.dev.conf` | Header WebSocket untuk HMR Metro. **Hanya dev.** |
| `frontend-upgrade.prod.conf` | Sengaja kosong. Jangan dihapus, karena Dockerfile tetap meng-COPY-nya. |
| `Dockerfile` | Image nginx 1.27 Alpine. Menyalin berkas di atas ke `/etc/nginx/`. |
| `Dockerfile.dockerignore` | Daftar putih berkas yang masuk build context. Berkas baru harus ditambahkan di sini. |
| `../../frontend/nginx.conf` | nginx di kontainer frontend (menyajikan hasil `expo export`), berbeda dari berkas di folder ini. |

## Argumen build (`Dockerfile`)

| ARG | Dev (bawaan) | Prod |
|---|---|---|
| `NGINX_CONF` | `nginx.conf` | `nginx.prod.conf` |
| `FRONTEND_UPGRADE` | `frontend-upgrade.dev.conf` | `frontend-upgrade.prod.conf` |

Dua ARG ini harus dipasangkan benar. Nilai prod diatur di `docker-compose.prod.yml` (`build.args`).

## Nama host dan port upstream

Nama host di `server.conf` adalah nama service di docker-compose:

| Variabel | Upstream | Keterangan |
|---|---|---|
| `$qms_backend` | `backend:8080` | API, SSE, health |
| `$qms_frontend` | `frontend:80` | Aplikasi web |
| `$qms_payment` | `payment-service:8093` | Pembayaran (terpisah dari antrian) |

`qr-service` (port 8090) tidak punya rute publik. `/internal/` selalu dibalas 404, dan backend memanggilnya langsung lewat
jaringan Docker. Upstream memakai variabel dan `resolver 127.0.0.11`, sehingga nginx tetap bisa start walaupun sebuah
service belum jalan, dan tidak 502 terus setelah service direstart.

## Routing (`server.conf`)

Urutan `location` penting: regex diperiksa lebih dulu daripada prefix, dan `location /` ada paling bawah.

| Path | Tujuan | Batas laju / catatan |
|---|---|---|
| `/api/v1/queues/:id/event` | backend | SSE: tanpa buffer, timeout 86400s |
| `/api/v1/users/login`, `/register` | backend | `auth_zone` 10 r/s, burst 20 |
| `/api/v1/users/forgot-password`, `/reset-password` | backend | `reset_zone` 1 r/s, burst 5 (batas per akun ada di backend) |
| `/reset-password` (halaman) | frontend | Token disamarkan di log, `Referrer-Policy no-referrer`, `no-store` |
| `/api/v1/display/:token/event` | backend | SSE tanpa buffer, `display_zone` 20 r/s, burst 40, token disamarkan di log |
| `/api/v1/display/:token` | backend | `display_zone`, token disamarkan di log |
| `/display/:token` (halaman) | frontend | Sama seperti `/reset-password` |
| `/api/v1/join`, `/api/v1/join/preview` | backend | `join_zone` 10 r/s, burst 20 |
| `/api/v1/users/me/avatar` | backend | Body sampai 6 MB (hanya rute ini), `avatar_zone` 5 r/s, burst 10 |
| `/api/v1/payments/webhooks/stripe` | payment-service | Tanpa batas laju (tanda tangan diverifikasi di service) |
| `/api/v1/payments/` | payment-service | `payment_zone` 2 r/s, burst 5 |
| `/health` | backend | Log akses dimatikan |
| `/api/` | backend | Timeout 60s |
| `/internal/` | - | Selalu 404 |
| `/` | frontend | Aplikasi web (fallback) |

Bila `payment-service` mati, `/api/v1/payments/` dan webhook dibalas JSON `PAYMENT_UNAVAILABLE` dengan status 503 dan
`Retry-After: 30`. Jalur antrian, login, dan SSE tidak terpengaruh.

Batas ukuran body default `client_max_body_size 1m` berlaku untuk semua rute, kecuali avatar.

Header proxy umum (`Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`) dan header keamanan dasar
(`X-Content-Type-Options`, `Referrer-Policy`) diatur di `server.conf`, yang berada di dalam `server{}`.

## Yang disamarkan di log

Token reset password ada di query string (`?token=`), dan token layar publik ada di path. Format log `qms_masked`
hanya mencatat `$uri_masked` (token diganti `***`), dan tidak pernah mencatat query string. Rute yang membawa token
memakai `access_log ... qms_masked`.

## Saat pindah domain atau topologi

| Yang berubah | Ubah di |
|---|---|
| Domain publik (browser dan HP) | `QMS_PUBLIC_ORIGIN` di `.env`. Harus sama dengan alamat yang dibuka. Backend dan qr-service memakai nilai ini untuk CORS dan `join_url`. |
| Domain di produksi | Konfigurasi tunnel Cloudflare, dan `server_name _` di `nginx.prod.conf` bila ingin menolak Host lain. |
| Nama atau port service | `set $qms_*` di `server.conf`, dan service di `docker-compose*.yml`. |
| Subnet jaringan | `set_real_ip_from` di `nginx.prod.conf`, `TRUSTED_PROXIES` backend di `docker-compose*.yml` (dev `172.28.0.0/16`, prod `172.29.0.0/16`), dan blok `ipam` di compose. Ketiganya harus sama. |
| Port yang dipetakan ke host | `EXPOSE` di `Dockerfile` (harus tetap 80) dan `QMS_HTTP_PORT` di compose. |
| Batas laju atau ukuran body | `limit_req_zone` di `nginx.conf` dan `nginx.prod.conf`, `limit_req` dan `client_max_body_size` di `server.conf`. Nama zona harus cocok. |

## Menjalankan

```bash
cp .env.example .env        # isi secret dan QMS_PUBLIC_ORIGIN
docker compose up --build   # nginx terbuka di ${QMS_HTTP_PORT:-8080}
```

Hanya ada satu `docker-compose.yml` (mode development, hot reload). Hanya nginx yang membuka port; `db`, `backend`,
`qr-service`, `payment-service`, dan `frontend` hanya terjangkau lewat jaringan Docker. Psql:
`docker compose exec db psql -U qms -d qms_db`.

`QMS_PUBLIC_ORIGIN` harus sama dengan alamat yang dibuka browser dan HP. Frontend memanggil API di origin halaman,
jadi tidak ada yang dibakar ke bundle.

`backend` memakai `TRUSTED_PROXIES` (subnet jaringan compose) agar IP klien dari `X-Forwarded-For` hanya dipercaya bila
datang dari nginx.

## nginx frontend

Satu HTML per rute dari Expo (mis. `dashboard.html`), jadi `try_files $uri $uri.html $uri/ /index.html`. Aset statis di-cache
setahun, HTML tidak di-cache. Berkasnya di `frontend/nginx.conf`.

## Produksi/demo: `nginx.prod.conf`

Di belakang Cloudflare Tunnel (`cloudflared` di host, target `http://localhost:8080`). Port nginx hanya terikat ke
`127.0.0.1`, sehingga hanya `cloudflared` yang bisa mengirim header `CF-Connecting-IP` yang dipercaya. Domain yang dipakai
diatur di Cloudflare. Header WebSocket Metro tidak ada di prod (`frontend-upgrade.prod.conf` kosong). Tunnel dikelola
manual, lihat `documentation/docs/setup-perintah-manual.md` (satu tingkat di atas `queue-management-system/`).

## HTTPS publik untuk uji di HP (quick tunnel, sementara)

HP tidak perlu memasang apa pun: Cloudflare quick tunnel memberi URL `https://*.trycloudflare.com` dengan sertifikat
publik yang dipercaya, jadi kamera pemindai QR langsung jalan.

```bash
# pasang cloudflared: https://github.com/cloudflare/cloudflared/releases
make tunnel   # buka tunnel, jalankan stack, cetak URL
make down     # hentikan
make mailpit  # tampilkan link Mailpit (dev) tanpa menjalankan apa pun
```

- URL acak dan berganti tiap sesi. Skrip memberikannya sebagai `QMS_PUBLIC_ORIGIN` ke backend dan qr-service, yang
  membacanya saat start.
- Butuh internet. Jangan dipakai untuk data nyata, karena siapa pun yang tahu URL-nya bisa mengakses aplikasi.
- Jaringan yang memblokir QUIC ditangani dengan `--protocol http2`.

## Belum ada

- HSTS (ditunda, keputusan 31 Okt; bila dipasang mulai dengan `max-age=300`, hanya di `nginx.prod.conf`). Sertifikat
  ditangani Cloudflare, bukan nginx.
- Mailpit lewat nginx (`/mail/`). Mailpit hanya terjangkau di `127.0.0.1` (lihat `make mailpit`). `mail-service`
  mengirim email ke Mailpit lewat jaringan internal (`SMTP_HOST: mailpit`).

## Uji cepat

- `/` menampilkan halaman login, dan `/dashboard` tidak 404.
- `/api/v1/queues` tanpa token membalas 401 (502 berarti backend mati).
- `/internal/v1/join-token` membalas 404.
- `/api/v1/payments/checkout` tanpa token membalas 401 dari payment-service (503 `PAYMENT_UNAVAILABLE` bila payment-service mati).
- Banyak request cepat ke `/api/v1/users/login` akhirnya dibalas 429.
- Owner menekan Next dan layar customer berubah tanpa refresh (SSE tidak tertahan buffer).
