# Konfigurasi nginx (QMS)

Folder ini berisi konfigurasi nginx sebagai reverse proxy. Ia **terpisah dari kode aplikasi**. Ada dua jalur pemakaian:
- **Di host** (langkah di bawah): `qms-http.conf` dan `qms.conf` dipasang ke `/etc/nginx`.
- **Di container**: `Dockerfile` di folder ini memakai `templates/default.conf.template`. Belum di-build dan belum ada `docker-compose.yml`.

## Isi

| Berkas | Untuk | Kapan dipakai |
|---|---|---|
| `qms-http.conf` | HTTP saja | Uji awal, dan agar certbot bisa memverifikasi domain |
| `qms.conf` | HTTPS + redirect dari HTTP | Setelah sertifikat terpasang (produksi) |
| `templates/default.conf.template` | HTTP untuk container, variabel diisi saat start | Dipakai `Dockerfile` |
| `Dockerfile`, `Dockerfile.dockerignore` | Image nginx | Belum di-build |

Kedua berkas punya dua placeholder yang harus diganti:
- `QMS_DOMAIN`: nama domain, mis. `qms.contoh.ac.id`.
- `QMS_FRONTEND_DIR`: folder hasil ekspor frontend, mis. `/var/www/qms/frontend`.

Alamat backend ada di blok `upstream qms_backend` (default `127.0.0.1:8080`).

## Yang diatur nginx

- **Frontend statis** dilayani dari `QMS_FRONTEND_DIR`. Rute Expo (`/dashboard`, `/queues/12`, dst.) jatuh kembali ke `index.html`.
- **API** `/api/...` diteruskan ke backend. Dengan begitu frontend dan API satu origin.
- **SSE** `/api/v1/queues/:id/event` diteruskan tanpa buffer dan dengan timeout 1 jam. Tanpa ini, event `ticket_next` tertahan di nginx dan layar customer tidak berubah secara live.
- **HTTPS** dengan redirect dari HTTP.

## Variabel untuk container

Dipakai hanya oleh `Dockerfile` (lewat `templates/default.conf.template`). Diisi saat `docker run`:

| Variabel | Default di image | Isi |
|---|---|---|
| `QMS_DOMAIN` | `localhost` | Nama host untuk `server_name` |
| `BACKEND_HOST` | `backend` | Nama service backend di jaringan Docker (port 8080) |
| `FRONTEND_ROOT` | `/srv/qms-frontend` | Folder frontend statis di dalam container (volume yang berisi `dist`) |

Container ini hanya melayani HTTP di port 80. TLS perlu diurus di luar container.

## Langkah pemasangan (host, tanpa Docker)

1. Build frontend di mesin build. `EXPO_PUBLIC_API_URL` **wajib** diisi dengan origin nginx (mis. `https://qms.contoh.ac.id`). Kalau kosong, frontend memanggil `host:8080` langsung dan melewati nginx:
   ```bash
   cd frontend
   EXPO_PUBLIC_API_URL=https://QMS_DOMAIN npx expo export --platform web
   ```
   Salin folder `dist/` ke server, mis. `/var/www/qms/frontend`.
2. Jalankan backend sebagai service di `127.0.0.1:8080` dengan `.env` produksi. Tambahkan origin produksi ke `ALLOWED_ORIGINS` (mis. `https://qms.contoh.ac.id`).
3. Salin `qms-http.conf` ke `/etc/nginx/sites-available/qms`, ganti placeholder, aktifkan, lalu cek:
   ```bash
   sudo ln -s /etc/nginx/sites-available/qms /etc/nginx/sites-enabled/qms
   sudo nginx -t && sudo systemctl reload nginx
   ```
4. Ambil sertifikat:
   ```bash
   sudo certbot certonly --webroot -w /var/www/certbot -d QMS_DOMAIN
   ```
5. Ganti isi berkas dengan `qms.conf` (placeholder diganti dengan cara yang sama), lalu `nginx -t` dan reload.

## Uji cepat setelah pemasangan

- `https://QMS_DOMAIN/` menampilkan halaman login.
- `https://QMS_DOMAIN/api/v1/queues` tanpa token membalas 401 (bukan 502). 502 berarti backend tidak jalan.
- Buka dua browser, owner dan customer. Saat owner menekan Next, layar customer berubah tanpa refresh. Kalau tidak berubah, periksa blok SSE.

## Catatan

- Karena lewat proxy, frontend dan API berada di satu origin. CORS hanya dibutuhkan kalau frontend diakses dari origin lain, tetapi menambahkan origin produksi ke `ALLOWED_ORIGINS` tetap aman.
- Sertifikat diperpanjang otomatis oleh certbot. Blok `/.well-known/acme-challenge/` di `qms.conf` harus tetap ada agar perpanjangan berhasil.
- Backend memakai `engine.Run(port)`, jadi jangan membuka port 8080 ke publik. Biarkan hanya nginx yang terbuka (80 dan 443).
