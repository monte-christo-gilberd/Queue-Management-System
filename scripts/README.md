# scripts/

Skrip bantu untuk menjalankan, memeriksa, dan merawat stack QMS. Semua skrip **dijalankan dari akar
`queue-management-system/`** (mereka sendiri pindah ke akar repo lewat `cd "$(dirname "$0")/.."`, jadi path relatif
aman). Sebagian besar dipanggil lewat `make`; yang lain dijalankan langsung.

Prasyarat umum: `bash`, `docker` dengan `docker compose`. `check-service-isolation.sh` juga butuh `python3`;
`gen-auth-keys.sh` dan `create-demo-accounts.sh` butuh `openssl`; `create-demo-accounts.sh` butuh `curl`.

| Skrip | Untuk apa | Stack | Dipanggil oleh |
|---|---|---|---|
| [`gen-auth-keys.sh`](#gen-auth-keyssh) | Membuat pasangan kunci RSA untuk token login | dev dan prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-service-isolation.sh`](#check-service-isolationsh) | Menjaga isolasi antar service di compose (termasuk overlay observability) | dev dan prod | `make obs-check`, manual |
| [`check-observability-leaks.sh`](#check-observability-leakssh) | Tes kebocoran rahasia ke log, trace, dan metrik | dev dan prod (overlay observability) | `make obs-check` |
| [`check-google-config.sh`](#check-google-configsh) | Memeriksa konfigurasi login Google (server dan frontend sinkron) | dev dan prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-mail-mode.sh`](#check-mail-modesh) | Memeriksa `MAIL_MODE` dan `SMTP_*` sebelum stack prod naik | prod | `make prod-up` |
| [`verify-user.sh`](#verify-usersh) | Menandai email satu akun terverifikasi langsung di database | dev atau prod | `make verify-user` |
| [`create-demo-accounts.sh`](#create-demo-accountssh) | Membuat 1 owner dan 4 customer demo | prod | manual |
| [`reset-demo.sh`](#reset-demosh) | Mengosongkan antrian dan tiket demo, akun dipertahankan | prod | `make prod-reset` |
| [`seed-prod-penguji.sh`](#seed-prod-penguji-dan-restore-prod-penguji) | Membuat 1 akun penguji (owner + customer) dan mengisi data uji antrian | prod | `make prod-seed-penguji` |
| [`restore-prod-penguji.sh`](#seed-prod-penguji-dan-restore-prod-penguji) | Mengembalikan data uji akun penguji ke kondisi seed | prod | `make prod-restore-penguji` |
| [`backup-prod.sh`](#backup-prodsh) | Backup dan restore database prod | prod | `make prod-backup`, `make prod-reset` |

Variabel lingkungan yang dibaca beberapa skrip:

| Variabel | Arti | Bawaan |
|---|---|---|
| `QMS_ENV_FILE` | File env yang dipakai compose prod | `.env.prod` (`.env` untuk `backup-prod.sh` dan `check-service-isolation.sh`) |
| `POSTGRES_USER`, `POSTGRES_DB` | Pengguna dan nama database | `qms`, `qms_db` |

---

## gen-auth-keys.sh

Membuat pasangan kunci RSA 2048 bit untuk token login (JWT RS256). Prod memakai folder `keys/`: idempoten, kunci tidak diganti oleh `make prod-up`. Dev memakai folder terpisah `keys-dev/` (`AUTH_KEYS_DIR=keys-dev`): `make up` dan `make tunnel` membuat kunci baru (menghapus `keys-dev/` dulu), `make down` menghapusnya. Kunci prod tidak pernah disentuh target dev.

| Berkas | Isi | Dipasang ke |
|---|---|---|
| `auth-private.pem` | Kunci privat | **Hanya** `auth-service` (penerbit token) |
| `auth-public.pem` | Kunci publik | `backend` (verifikasi lokal) dan `auth-service` |

Berkas ada di `keys/` (prod) atau `keys-dev/` (dev).

```bash
scripts/gen-auth-keys.sh
```

- **Idempoten.** Bila kedua kunci sudah ada, tidak ada yang ditimpa dan skrip keluar dengan kode 0. Menimpa kunci
  membuat semua token login yang beredar tidak sah.
- Bila hanya satu kunci yang ada, skrip berhenti dengan galat (keadaan setengah jadi, tidak ditebak).
- Dipanggil otomatis oleh `make up`, `make tunnel`, dan `make prod-up`, jadi biasanya tidak perlu dijalankan manual.
- Folder `keys/` dan `keys-dev/` ada di `.gitignore`. Jangan di-commit.
- `AUTH_KEYS_DIR=<folder>` mengganti lokasi (bawaan `keys`).

**Rotasi kunci:** hapus kedua berkas, jalankan skrip lagi, lalu `make restart` (atau restart `backend`,
`auth-service`). Semua user harus login ulang.

---

## check-service-isolation.sh

Memeriksa konfigurasi `docker-compose.yml` dan `docker-compose.prod.yml` serta kode, untuk menjaga jaminan berikut.
Keluar dengan kode 1 dan pesan `GAGAL [...]` bila ada yang dilanggar.

| Pemeriksaan | Jaminan |
|---|---|
| `notify-service` | Tidak bergantung pada `mail-service`, Mailpit, atau SMTP. Email mati tidak boleh menghentikan push |
| `auth-service` | Tidak ada service lain yang punya `depends_on` ke sana, ia hanya bergantung pada `db`, dan **kunci privat JWT hanya di-mount ke `auth-service`**. Karena itu user yang sudah login tetap bisa memakai aplikasi saat ia mati |
| Kode | `notify-service` tidak menyebut smtp atau mailer. `backend` tidak memanggil `auth-service` |
| nginx | Upstream `auth-service` lewat variabel + resolver, supaya nginx tetap start bila salah satunya mati |
| Observability: overlay | Dengan `docker-compose.observability.yml` (dev dan prod): tanpa `depends_on` dari service aplikasi ke komponen observability, hanya Grafana yang membuka port ke host (dan hanya di `127.0.0.1`), port internal (9100, 8081, 4317, 9090, 3100, 3200) tidak dipublikasikan, tanpa `docker.sock`, mount direktori host read-only, semua komponen punya `mem_limit`, `OTEL_EXPORTER_OTLP_ENDPOINT` hanya untuk backend, auth, qr, mail dan hanya ke collector |
| Observability: compose dasar | Tanpa overlay, compose dasar tidak punya service observability dan tidak membawa variabel `OTEL_*`, jadi aplikasi start apa adanya |
| Observability: nginx dan kode | nginx tidak meneruskan `/metrics` atau `:9100`, listener `stub_status` `:8081` punya `deny all`, dan kode Go tidak menulis alamat komponen observability secara keras |

```bash
scripts/check-service-isolation.sh
QMS_ENV_FILE=.env.prod scripts/check-service-isolation.sh
```

Jalankan setiap kali mengubah `docker-compose*.yml` atau `deploy/nginx/*.conf`. Juga dijalankan oleh `make obs-check`.
Compose prod butuh variabel wajib terisi; skrip memakai `${QMS_ENV_FILE:-.env}` dan mengisi nilai SMTP palsu untuk
keperluan parsing saja.

---

## check-observability-leaks.sh

Tes kebocoran rahasia ke observability (tujuan 6 di `documentation/docs/plan-observability-tracing.md`). Mengirim request
yang ditolak (404/401/400) lewat nginx dengan **rahasia palsu yang unik per eksekusi**: token layar publik (path dan query),
password dan email penuh di body login, token verifikasi email, token reset password, dan JWT palsu di header
`Authorization`. Setelah pipeline sempat memproses (sekitar 45 detik), skrip mencari rahasia itu di:

| Tempat | Cara |
|---|---|
| Log (Loki) | Pencarian teks bebas di semua service |
| Trace (Tempo) | Mengambil trace terbaru lalu memindai seluruh isi span (atribut, event, status) |
| Metrik (Prometheus) | Memindai semua label metrik `http_*`, `auth_*`, `nginx_*` |

Kode keluar: `0` bersih, `1` ada kebocoran (rahasia dan tempatnya dicetak), `2` stack observability tidak terjangkau,
`3` tidak ada log nginx baru di Loki (pemeriksaan tidak berarti). Skrip tidak membuat data; hanya menambah beberapa angka
di metrik login gagal dan 404.

```bash
scripts/check-observability-leaks.sh
QMS_ENV_FILE=.env.prod scripts/check-observability-leaks.sh
LEAK_ROUNDS=80 scripts/check-observability-leaks.sh                 # lebih pasti ada trace yang terekam di sampling 10%
LEAK_EXTRA_CANARY="teks-uji" scripts/check-observability-leaks.sh   # kontrol positif: harus GAGAL bila "teks-uji" ada di log
```

Prasyarat: stack jalan dengan overlay (`make up obs`), `curl`, `openssl`, `python3`. Membaca `GRAFANA_ADMIN_PASSWORD`,
`GRAFANA_PORT`, dan `QMS_HTTP_PORT` dari `.env` (atau `QMS_ENV_FILE`). Dijalankan juga oleh `make obs-check`.

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
`tickets`, `queue_history`, `queue_settings`, `queue_displays`, dan `queues`. Akun (`users`, `owners`, `payments`, foto
profil) tetap, dan langganan owner diperpanjang 30 hari. Backup dibuat otomatis lebih dulu.

Semua tabel yang mereferensikan `queues`, `tickets`, atau `counters` harus ikut di perintah `TRUNCATE`, karena skrip tidak
memakai `CASCADE`. Tabel yang terlewat membuat Postgres menolak seluruh perintah dan transaksi dibatalkan (itu yang terjadi
sebelum `queue_displays` ditambahkan).

```bash
scripts/reset-demo.sh        # minta konfirmasi (ketik 'ya')
scripts/reset-demo.sh -y     # tanpa konfirmasi
make prod-reset
```

Tidak menyentuh stack dev. Gagal dengan pesan jelas bila stack prod tidak berjalan. Setelah selesai, skrip mencetak
jumlah akun dan owner yang tersisa.

---

## seed-prod-penguji dan restore-prod-penguji

Data uji untuk penguji yang hanya punya **satu akun** (`penguji@montechristo.my.id`, username `penguji`). Akun itu
adalah owner sekaligus customer, sehingga semua fitur antrian bisa diuji dari satu login:

- **Sebagai owner** (antrian aktif "Antrian Uji Penguji"): NEXT, panggil ulang, skip, hapus nomor, sisip pelanggan
  (lewat username `pelanggan-9` atau `pelanggan-10`), catatan, counter (Loket 1 dan Loket 2), tutup antrian, QR, layar
  publik, riwayat dan ekspor (antrian "Antrian Uji Sebelumnya").
- **Sebagai customer**: bergabung dengan memindai QR antriannya sendiri lewat HP (akun yang sama), batalkan nomor,
  Antrian Saya, Riwayat Saya, dan notifikasi.

Isi data uji ada di [`db/scripts/seed_prod_penguji.sql`](../db/scripts/seed_prod_penguji.sql). Pelanggan seed
(`pelanggan-1` sampai `pelanggan-10`) adalah akun tanpa kata sandi, tidak bisa login, dan notifikasi emailnya mati.

```bash
make prod-seed-penguji        # atau: scripts/seed-prod-penguji.sh [-y]
make prod-restore-penguji     # atau: scripts/restore-prod-penguji.sh [-y]
```

- **seed**: bila akun belum ada, mendaftarkannya lewat API dengan sandi acak 16 karakter. Kredensial ditulis ke
  `~/qms-penguji-account.txt` (mode 600, di luar repo) dan tidak menimpa berkas yang ada. Bila akun sudah ada, sandinya
  **tidak** diubah dan hanya datanya yang diisi ulang. Email dan username bisa diganti lewat `PENGUJI_EMAIL` dan
  `PENGUJI_USERNAME`. Email verifikasi dari pendaftaran ikut terkirim sesuai `MAIL_MODE`.
- **restore**: mengembalikan antrian, tiket, counter, layar publik (token baru), catatan, riwayat, dan profil dasar
  (username, notifikasi email, foto profil, langganan 365 hari dari sekarang) ke kondisi seed. Akun dan sandinya
  tetap. Akun harus sudah ada (`seed` dulu).
- Keduanya membuat backup database (`scripts/backup-prod.sh`) lebih dulu dan meminta konfirmasi (`ketik 'ya'`) kecuali
  memakai `-y`.
- Hanya akun penguji, tiketnya, dan akun pelanggan seed yang disentuh. Akun lain dan data produksi lain tidak.
- Jalankan restore **setelah** sesi pengujian selesai. Bila `make prod-reset` dijalankan, antrian penguji ikut
  terhapus, jadi jalankan `make prod-restore-penguji` sesudahnya. Restore tidak mengembalikan perubahan sandi; gunakan
  alur lupa sandi bila penguji mengubahnya.
- Kode QR dan token layar publik berbeda setiap kali data diisi ulang. Cetak ulang QR dari layar owner dan gunakan
  tautan layar publik yang ditampilkan saat skrip selesai.

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
`db/scripts/backfill_currently_served.sql`. Role per service dibuat oleh `db/migrations/03_notify_role.sh`, `04_mail_role.sh`, dan `06_auth_role.sh`; untuk
database lama yang belum punya role `auth_rw`, jalankan `06_auth_role.sh` seperti dijelaskan di `.env.example`.
