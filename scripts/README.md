# scripts/

Skrip bantu untuk menjalankan, memeriksa, dan merawat stack QMS. Semua skrip **dijalankan dari akar
`queue-management-system/`** (mereka sendiri pindah ke akar repo lewat `cd "$(dirname "$0")/.."`, jadi path relatif
aman). Sebagian besar dipanggil lewat `make`; yang lain dijalankan langsung.

Prasyarat umum: `bash`, `docker` dengan `docker compose`. `check-service-isolation.sh` juga butuh `python3`;
`gen-auth-keys.sh` dan `create-demo-accounts.sh` butuh `openssl`; `create-demo-accounts.sh` butuh `curl`.

| Skrip | Untuk apa | Stack | Dipanggil oleh |
|---|---|---|---|
| [`gen-auth-keys.sh`](#gen-auth-keyssh) | Membuat pasangan kunci RSA untuk token login | dev dan prod | `make up`, `make prod-up` |
| [`check-service-isolation.sh`](#check-service-isolationsh) | Menjaga isolasi antar service di compose | dev dan prod | manual |
| [`check-google-config.sh`](#check-google-configsh) | Memeriksa konfigurasi login Google (server dan frontend sinkron) | dev dan prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-mail-mode.sh`](#check-mail-modesh) | Memeriksa `MAIL_MODE` dan `SMTP_*` sebelum stack prod naik | prod | `make prod-up` |
| [`verify-user.sh`](#verify-usersh) | Menandai email satu akun terverifikasi langsung di database | dev atau prod | `make verify-user` |
| [`create-demo-accounts.sh`](#create-demo-accountssh) | Membuat 1 owner dan 4 customer demo | prod | manual |
| [`reset-demo.sh`](#reset-demosh) | Mengosongkan antrian dan tiket demo, akun dipertahankan | prod | `make prod-reset` |
| [`backup-prod.sh`](#backup-prodsh) | Backup dan restore database prod | prod | `make prod-backup`, `make prod-reset` |

Variabel lingkungan yang dibaca beberapa skrip:

| Variabel | Arti | Bawaan |
|---|---|---|
| `QMS_ENV_FILE` | File env yang dipakai compose prod | `.env.prod` (`.env` untuk `backup-prod.sh` dan `check-service-isolation.sh`) |
| `POSTGRES_USER`, `POSTGRES_DB` | Pengguna dan nama database | `qms`, `qms_db` |

---

## gen-auth-keys.sh

Membuat pasangan kunci RSA 2048 bit untuk token login (JWT RS256) di folder `keys/`. Dev: `make up` dan `make tunnel` membuat kunci baru (menghapus `keys/` dulu), `make down` menghapusnya. Prod: idempoten, kunci tidak diganti oleh `make prod-up`.

| Berkas | Isi | Dipasang ke |
|---|---|---|
| `keys/auth-private.pem` | Kunci privat | **Hanya** `auth-service` (penerbit token) |
| `keys/auth-public.pem` | Kunci publik | `backend` dan `payment-service` (verifikasi lokal), serta `auth-service` |

```bash
scripts/gen-auth-keys.sh
```

- **Idempoten.** Bila kedua kunci sudah ada, tidak ada yang ditimpa dan skrip keluar dengan kode 0. Menimpa kunci
  membuat semua token login yang beredar tidak sah.
- Bila hanya satu kunci yang ada, skrip berhenti dengan galat (keadaan setengah jadi, tidak ditebak).
- Dipanggil otomatis oleh `make up` dan `make prod-up`, jadi biasanya tidak perlu dijalankan manual.
- Folder `keys/` ada di `.gitignore`. Jangan di-commit.
- `AUTH_KEYS_DIR=<folder>` mengganti lokasi (bawaan `keys`).

**Rotasi kunci:** hapus kedua berkas, jalankan skrip lagi, lalu `make restart` (atau restart `backend`,
`auth-service`, `payment-service`). Semua user harus login ulang.

---

## check-service-isolation.sh

Memeriksa konfigurasi `docker-compose.yml` dan `docker-compose.prod.yml` serta kode, untuk menjaga jaminan berikut.
Keluar dengan kode 1 dan pesan `GAGAL [...]` bila ada yang dilanggar.

| Pemeriksaan | Jaminan |
|---|---|
| `notify-service` | Tidak bergantung pada `mail-service`, Mailpit, atau SMTP. Email mati tidak boleh menghentikan push |
| `payment-service` | Terpisah dari jalur antrian. Hanya bergantung pada `db`. Simulator pembayaran palsu tidak ada di prod |
| `auth-service` | Tidak ada service lain yang punya `depends_on` ke sana, ia hanya bergantung pada `db`, dan **kunci privat JWT hanya di-mount ke `auth-service`**. Karena itu user yang sudah login tetap bisa memakai aplikasi saat ia mati |
| Kode | `notify-service` tidak menyebut smtp atau mailer. `backend` tidak menyebut payment-service atau Stripe, dan tidak memanggil `auth-service` |
| nginx | Upstream `payment-service` dan `auth-service` lewat variabel + resolver, supaya nginx tetap start bila salah satunya mati |

```bash
scripts/check-service-isolation.sh
QMS_ENV_FILE=.env.prod scripts/check-service-isolation.sh
```

Jalankan setiap kali mengubah `docker-compose*.yml` atau `deploy/nginx/*.conf`. Skrip ini tidak ada di Makefile.
Compose prod butuh variabel wajib terisi; skrip memakai `${QMS_ENV_FILE:-.env}` dan mengisi nilai SMTP palsu untuk
keperluan parsing saja.

---

## check-google-config.sh

Memeriksa konfigurasi login Google di file env sebelum stack dijalankan. Dipanggil otomatis oleh `make up` dan `make tunnel`
(membaca `.env`) dan `make prod-up` (membaca `.env.prod`).

| Keadaan | Hasil |
|---|---|
| `GOOGLE_CLIENT_IDS` dan `EXPO_PUBLIC_GOOGLE_*` semua kosong | Login Google mati (tombol tersembunyi, rute 404). **Bukan galat** |
| Client ID frontend **tidak ada** di `GOOGLE_CLIENT_IDS` | **GAGAL (exit 1)**: setiap ID token ditolak server (401, `aud` tidak cocok) |
| `EXPO_PUBLIC_GOOGLE_*` terisi tetapi `GOOGLE_CLIENT_IDS` kosong | **GAGAL (exit 1)**: tombol tampil tetapi server menjawab 404 |
| `GOOGLE_CLIENT_IDS` terisi tetapi frontend kosong | Peringatan: tombol tidak akan tampil |
| Client ID tidak berakhiran `.apps.googleusercontent.com` | Peringatan: kemungkinan nilai contoh atau salah ketik |

```bash
scripts/check-google-config.sh            # memeriksa .env
scripts/check-google-config.sh .env.prod  # file env eksplisit
```

Skrip ini hanya membaca file; tidak menjalankan apa pun dan tidak mencetak nilai client ID.

### Yang ikut dilakukan Makefile untuk login Google

| Target | Tambahan |
|---|---|
| `make up` | Memeriksa konfigurasi; **membersihkan cache Metro otomatis** bila `EXPO_PUBLIC_GOOGLE_*` di `.env` berubah sejak build terakhir (penanda `.metro-google.stamp`, tidak ikut git); mencetak pengingat bahwa Google hanya jalan di `http://localhost:8080`, bukan alamat IP LAN |
| `make tunnel` | Sama, dan mencetak origin tunnel yang harus didaftarkan di Google Cloud (URL berubah tiap sesi, jadi harus didaftarkan ulang setiap kali) |
| `make prod-up` | Memeriksa `.env.prod`; mencetak origin https yang harus terdaftar dan pengingat memperbarui role `auth_rw` pada database yang sudah ada |

Cache Metro perlu dibersihkan karena Metro menyimpan nilai `EXPO_PUBLIC_*` di cache transformasi, sehingga mengubahnya di `.env` tanpa membersihkan cache membuat bundle memakai nilai lama. Build prod tidak mengalaminya: nilainya masuk sebagai argumen build Docker, yang membuat layer dibangun ulang sendiri.

---

## check-mail-mode.sh

Memeriksa `MAIL_MODE` di file env prod sebelum stack naik. Dipanggil otomatis oleh `make prod-up`.

| `MAIL_MODE` | Arti | Pemeriksaan |
|---|---|---|
| `mailpit` (**bawaan bila kosong**) | Email ditangkap Mailpit dan **tidak** dikirim ke siapa pun (demo atau uji) | Hanya peringatan: UI Mailpit hanya di `127.0.0.1`, dan bila verifikasi email menyala, user baru tidak menerima tautan |
| `smtp` | Email dikirim lewat SMTP nyata | `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` terisi dan bukan nilai contoh (`GANTI`); host bukan `mailpit`; `SMTP_TLS=none` hanya untuk localhost |

Pemeriksaan umum untuk kedua mode: `MAIL_SERVICE_TOKEN` terisi, bukan contoh, minimal 32 karakter; `MAIL_DB_PASSWORD`
terisi dan bukan contoh.

```bash
scripts/check-mail-mode.sh                 # memakai $QMS_ENV_FILE atau .env.prod
scripts/check-mail-mode.sh .env.prod       # file env eksplisit
MAIL_MODE=smtp scripts/check-mail-mode.sh .env.prod
```

`MAIL_MODE` di lingkungan menimpa nilai di file. Kode keluar 1 bila konfigurasi salah.

---

## verify-user.sh

Menandai `users.email_verified_at` satu akun langsung di database, tanpa menunggu email. Berguna bila email
verifikasi tidak mungkin sampai: `MAIL_MODE=mailpit`, SMTP belum disiapkan, atau email salah ketik. Idempoten.

```bash
scripts/verify-user.sh budi@mail.com          # stack dev
scripts/verify-user.sh budi@mail.com prod     # stack prod (env dari $QMS_ENV_FILE, bawaan .env.prod)

make verify-user EMAIL=budi@mail.com          # lewat make
make verify-user EMAIL=budi@mail.com STACK=prod
```

Hasil: `OK` (diubah), `Sudah terverifikasi` (tidak ada yang diubah), atau `GAGAL` (email tidak ada, kode keluar 1).
Email dikirim sebagai variabel psql, bukan disisipkan ke teks SQL.

---

## create-demo-accounts.sh

Membuat 1 owner (langganan 30 hari) dan 4 customer lewat API register, dengan sandi acak 16 karakter. Semua akun
langsung ditandai terverifikasi (tidak bergantung pada kotak surat atau Mailpit).

```bash
scripts/create-demo-accounts.sh
QMS_API=http://localhost:8080 QMS_DEMO_FILE=~/akun.txt scripts/create-demo-accounts.sh
```

- Stack prod harus sudah berjalan (`make prod-up`). Registrasi lewat nginx dilayani `auth-service`.
- Daftar akun ditulis ke `~/qms-demo-accounts.txt` (mode 600, di luar repo). Skrip **menolak berjalan** bila berkas itu
  sudah ada; hapus dulu bila ingin membuat ulang.
- `QMS_API` (bawaan `http://localhost:8080`) dan `QMS_DEMO_FILE` mengganti alamat API dan lokasi keluaran.
- Domain di nama akun dan judul keluaran ditulis tetap di dalam skrip; sesuaikan bila domain demo berbeda.

---

## reset-demo.sh

Mengosongkan data demo di stack **prod**: `notification_outbox`, `push_subscriptions`, `ticket_notes`, `counters`,
`tickets`, `queue_history`, `queue_settings`, dan `queues`. Akun (`users`, `owners`, `payments`) tetap, dan langganan
owner diperpanjang 30 hari. Backup dibuat otomatis lebih dulu.

```bash
scripts/reset-demo.sh        # minta konfirmasi (ketik 'ya')
scripts/reset-demo.sh -y     # tanpa konfirmasi
make prod-reset
```

Tidak menyentuh stack dev. Gagal dengan pesan jelas bila stack prod tidak berjalan. Setelah selesai, skrip mencetak
jumlah akun dan owner yang tersisa.

---

## backup-prod.sh

Backup dan restore database prod memakai `pg_dump` dan `psql` di dalam kontainer `db`.

```bash
scripts/backup-prod.sh backup                  # ~/qms-backups/qms-<waktu>.sql.gz (di luar repo)
scripts/backup-prod.sh backup /path/file.sql.gz
scripts/backup-prod.sh restore /path/file.sql.gz
make prod-backup
```

- Berkas backup dibuat dengan `umask 077` (hanya terbaca pemilik). Backup kosong dianggap gagal dan dihapus.
- Dump memakai `--clean --if-exists`, jadi **restore menimpa isi database prod**. Skrip meminta Anda mengetik `ya`.
- File env dibaca dari `$QMS_ENV_FILE` (bawaan `.env`; untuk prod biasanya `QMS_ENV_FILE=.env.prod`, yang sudah
  diatur oleh `make prod-backup`).
- Tabel milik `auth-service` (`users`, `user_action_tokens`) ikut ter-backup karena berada di database yang sama.
  Kunci JWT (`keys/`) **tidak** ikut. Simpan cadangannya terpisah: tanpa kunci yang sama, token lama tidak sah, tetapi
  user tetap bisa login lagi dan akun tidak hilang.

---

## Skrip SQL di db/

Bukan di folder ini, tetapi sering dibutuhkan bersama skrip di atas: `db/scripts/apply_schema.sql` (penyesuaian skema
idempoten untuk database yang sudah berjalan), `db/scripts/reset_seed_dev.sql` (data seed dev, `make seed`), dan
`db/scripts/backfill_currently_served.sql`. Role per service dibuat oleh `db/migrations/03..06_*_role.sh`; untuk
database lama yang belum punya role `auth_rw`, jalankan `06_auth_role.sh` seperti dijelaskan di `.env.example`.
