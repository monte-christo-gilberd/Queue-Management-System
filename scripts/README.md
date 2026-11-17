# scripts/

Skrip bantu untuk menjalankan, memeriksa, dan merawat stack QMS. Semua skrip **dijalankan dari akar
`queue-management-system/`** (mereka sendiri pindah ke akar repo lewat `cd "$(dirname "$0")/.."`, jadi path relatif
aman). Sebagian besar dipanggil lewat `make`; yang lain dijalankan langsung.

Prasyarat umum: `bash`, `docker` dengan `docker compose`. `check-service-isolation.sh` juga butuh `python3`;
`gen-auth-keys.sh` dan `seed-prod-penguji.sh` butuh `openssl`; `seed-prod-penguji.sh` juga butuh `curl` dan modul Python `bcrypt`.

| Skrip | Untuk apa | Stack | Dipanggil oleh |
|---|---|---|---|
| [`gen-auth-keys.sh`](#gen-auth-keyssh) | Membuat pasangan kunci RSA untuk token login | dev dan prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-service-isolation.sh`](#check-service-isolationsh) | Menjaga isolasi antar service di compose (termasuk overlay observability) | dev dan prod | `make obs-check`, manual |
| [`check-observability-leaks.sh`](#check-observability-leakssh) | Tes kebocoran rahasia ke log, trace, dan metrik | dev dan prod (overlay observability) | `make obs-check` |
| [`check-google-config.sh`](#check-google-configsh) | Memeriksa konfigurasi login Google (server dan frontend sinkron) | dev dan prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-mail-mode.sh`](#check-mail-modesh) | Memeriksa `MAIL_MODE` dan `SMTP_*` sebelum stack prod naik | prod | `make prod-up` |
| [`check-payment-config.sh`](#check-payment-configsh) | Memeriksa kunci BorderPay, token webhook, token service, dan password role `payment_rw` (hanya keberadaan dan panjang, nilai tidak dicetak) | prod | `make prod-up`, `make prod-payment-check` |
| [`verify-user.sh`](#verify-usersh) | Menandai email satu akun terverifikasi langsung di database | dev atau prod | `make verify-user` |
| [`reset-demo.sh`](#reset-demosh) | RESET TOTAL database prod (semua akun dihapus) | prod | `make prod-reset` |
| [`seed-prod-penguji.sh`](#seed-prod-penguji) | Membuat 1 akun penguji (owner + customer) dan mengisi data uji antrian | prod | `make prod-seed-penguji` |
| [`backup-prod.sh`](#backup-prodsh) | Backup dan restore database prod | prod | `make prod-backup`, `make prod-reset`, `seed-prod-penguji.sh` |

Variabel lingkungan yang dibaca beberapa skrip:

| Variabel | Arti | Bawaan |
|---|---|---|
| `QMS_ENV_FILE` | File env yang dipakai compose prod | `.env.prod` (`.env` untuk `backup-prod.sh` dan `check-service-isolation.sh`) |
| `POSTGRES_USER`, `POSTGRES_DB` | Pengguna dan nama database (dibaca `backup-prod.sh`, `reset-demo.sh`, `seed-prod-penguji.sh`) | `qms`, `qms_db` |

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
| Observability: overlay | Dengan `docker-compose.observability.yml` (dev dan prod): tanpa `depends_on` dari service aplikasi ke komponen observability, hanya Grafana yang membuka port ke host (dan hanya di `127.0.0.1`), port internal (9100, 8081, 4317, 9090, 3100, 3200) tidak dipublikasikan, tanpa `docker.sock`, mount direktori host read-only, semua komponen punya `mem_limit`, `OTEL_EXPORTER_OTLP_ENDPOINT` hanya untuk backend, auth, mail dan hanya ke collector |
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

Tes kebocoran rahasia ke observability (tujuan 6 di `plan-observability-tracing.md`, folder `documentation/docs/` satu tingkat di atas `queue-management-system/`). Mengirim request
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
`GRAFANA_PORT`, dan `QMS_HTTP_PORT` dari `.env` (atau `QMS_ENV_FILE`). Variabel lain: `QMS_BASE_URL` (alamat nginx, bawaan `http://localhost:<QMS_HTTP_PORT>`) dan `LEAK_ROUNDS` (bawaan 40). Dijalankan juga oleh `make obs-check`.

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

## check-payment-config.sh

Memeriksa konfigurasi pembayaran BorderPay di file env prod. Dipanggil `make prod-up` (sebelum stack dinyalakan) dan
`make prod-payment-check` (kapan saja, tanpa mengubah apa pun). Skrip hanya mencetak keberadaan, panjang, dan prefix;
nilai rahasia tidak pernah tampil.

| Variabel | Pemeriksaan | Hasil bila gagal |
|---|---|---|
| `BORDERPAY_API_KEY` | Terisi, berawalan `bp_test_` atau `bp_live_` | Galat bila kosong atau prefix lain. `bp_test_` hanya peringatan: pembayaran sungguhan tidak akan diterima |
| `BORDERPAY_WEBHOOK_TOKEN` | Terisi, minimal 32 karakter, bukan contoh. Dianjurkan berawalan `bpt_` | Galat; `bpt_` hanya peringatan |
| `PAYMENT_SERVICE_TOKEN` | Terisi, minimal 32 karakter, bukan contoh | Galat (checkout dan webhook akan gagal) |
| `PAYMENT_DB_PASSWORD` | Terisi dan bukan contoh | Galat (payment-service tidak bisa konek ke database) |

```bash
scripts/check-payment-config.sh                # memakai $QMS_ENV_FILE atau .env.prod
scripts/check-payment-config.sh .env.prod      # file env eksplisit
```

Kode keluar 1 bila ada galat, sehingga `make prod-up` berhenti sebelum container pembayaran dibangun ulang.

Pemeriksaan hak database (`db/scripts/check_payment_grants.sql`) dijalankan terpisah oleh `make prod-up` dan
`make prod-payment-check`, karena butuh database yang sedang berjalan.

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
Email dikirim sebagai variabel psql, bukan disisipkan ke teks SQL. Argumen kedua hanya boleh `prod`; selain itu skrip keluar dengan kode 1.

---

## reset-demo.sh

**Reset total** database prod. Volume `qms-prod_db_data_prod` dihapus, lalu `make prod-up` menyalakan stack lagi dengan
database kosong. Container `db` membuat ulang skema dan role dari `db/migrations/`. **Semua akun (owner, pelanggan,
penguji), antrian, tiket, riwayat, dan pembayaran ikut hilang.**

Urutan skrip: cek stack berjalan, konfirmasi (ketik `hapus-prod`), backup (`backup-prod.sh`), `down`, hapus volume,
`make prod-up`, tunggu `db` sehat, lalu `CREATE TABLE _no_seed_guard` agar seed/reset dev ditolak di DB ini.

```bash
scripts/reset-demo.sh        # minta konfirmasi (ketik 'hapus-prod')
scripts/reset-demo.sh -y     # tanpa konfirmasi
make prod-reset
```

Setelah reset, buat ulang akun penguji dan data uji dengan `make prod-seed-penguji`. Untuk memulihkan data sebelumnya:
`scripts/backup-prod.sh restore <file>` (menimpa database prod).

## seed-prod-penguji

Data uji untuk penguji yang hanya punya **satu akun** (`penguji@montechristo.my.id`, username `penguji`). Akun itu
adalah owner sekaligus customer, sehingga semua fitur antrian bisa diuji dari satu login:

Setiap fitur punya antrian sendiri, dan setiap antrian aktif berisi 5 sampai 12 orang menunggu:

| Awalan | Antrian | Untuk diuji |
|---|---|---|
| N | Demo NEXT | NEXT berulang (10 menunggu), layar publik |
| U | Demo Panggil Ulang | PANGGIL ULANG pada nomor yang dilayani |
| S | Demo Skip | SKIP, pengaturan skip "kembali ke belakang" |
| H | Demo Hapus Nomor | Hapus nomor oleh owner (customer dapat email dan layarnya berubah) |
| B | Demo Batal Nomor | Batal dari customer. Akun penguji sudah punya nomor di sini (nomor 3) |
| I | Demo Sisip | Sisip pelanggan lewat username `pelanggan-7` sampai `pelanggan-12` |
| C | Demo Catatan | Catatan per nomor (sudah ada di nomor 1, 3, dan 5) |
| K | Demo Counter | CRUD counter. Loket 2 sedang melayani nomor 1 |
| T | Demo Tutup Antrian | Tutup antrian (pindah ke riwayat) |
| R | Riwayat Panjang | Antrian tutup dengan 25 nomor, untuk paginasi Riwayat dan ekspor |

Akun penguji juga bisa bergabung dengan memindai QR antriannya sendiri lewat HP (akun yang sama) untuk menguji sisi
customer: Antrian Saya, Riwayat Saya, dan notifikasi.

Isi data uji ada di [`db/scripts/seed_prod_penguji.sql`](../db/scripts/seed_prod_penguji.sql). Pelanggan seed
(`pelanggan-1` sampai `pelanggan-12`) punya kata sandi acak sendiri per run dan bisa login dengan username. Notifikasi emailnya mati.

Setiap run membuat kata sandi acak baru untuk akun penguji dan **semua** pelanggan seed, lalu menulis email, username, dan
kata sandinya ke `~/qms-penguji-account.txt` (mode 600, di luar repo, ditimpa setiap run). Sesi login lama semua akun itu dicabut. Jalankan ulang skrip yang sama untuk mengembalikan data uji ke kondisi seed.

```bash
make prod-seed-penguji        # atau: scripts/seed-prod-penguji.sh [-y]
```

- **seed**: setiap run membuat kata sandi acak 16 karakter untuk akun penguji. Bila akun belum ada, akun didaftarkan
  lewat API. Bila sudah ada, sandinya diganti lewat hash bcrypt. Kredensial ditulis ke `~/qms-penguji-account.txt`
  (mode 600, di luar repo) dan **ditimpa** setiap run. Setiap run juga mencabut sesi login lama akun penguji.
  Email dan username bisa diganti lewat `PENGUJI_EMAIL` dan `PENGUJI_USERNAME`. Email verifikasi dari pendaftaran
  ikut terkirim sesuai `MAIL_MODE`. `QMS_API` (bawaan `http://localhost:8080`) menentukan alamat API untuk pendaftaran, dan `PENGUJI_FILE` mengganti lokasi berkas kredensial. Skrip ini juga mengembalikan data uji ke kondisi seed, jadi tidak perlu
  skrip restore terpisah.
- Skrip membuat backup database (`scripts/backup-prod.sh`) lebih dulu dan meminta konfirmasi (`ketik 'ya'`) kecuali
  memakai `-y`. Butuh modul Python `bcrypt` di host.
- Hanya akun penguji, tiketnya, dan akun pelanggan seed yang disentuh. Akun lain dan data produksi lain tidak.
- Jalankan ulang seed **setelah** sesi pengujian selesai. Bila `make prod-reset` dijalankan, akun dan data penguji ikut
  terhapus, jadi jalankan `make prod-seed-penguji` sesudahnya. Sandi baru akan ditulis ke berkas kredensial.
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
`db/scripts/backfill_currently_served.sql`, dan `db/scripts/check_payment_grants.sql` (hanya membaca: memastikan role `payment_rw` punya semua hak yang dibutuhkan payment-service; dipanggil `make prod-up` dan `make prod-payment-check`). Role per service dibuat oleh `db/migrations/03_notify_role.sh`, `04_mail_role.sh`, `06_auth_role.sh`, dan `07_metrics_role.sh` (`metrics_ro` untuk postgres_exporter, dijalankan ulang oleh target observability); `08_payment_role.sh` untuk `payment_rw` juga dijalankan ulang oleh `make prod-up`. Untuk
database lama yang belum punya role `auth_rw`, jalankan `06_auth_role.sh` seperti dijelaskan di `.env.example`.

---

## Target Makefile terkait

Ringkasan target yang memanggil skrip di atas atau menggantikan langkah manual. `make up obs` (atau `obs=1`) menambahkan overlay observability.

| Target | Fungsi |
|---|---|
| `make up`, `make tunnel` | Stack dev (LAN atau Cloudflare quick tunnel). Membuat kunci `keys-dev/`, memeriksa Google, membersihkan cache Metro bila perlu |
| `make down` | Hentikan tunnel dan semua container dev, hapus `keys-dev/` dan penanda Metro |
| `make restart`, `make logs`, `make ps` | Restart `backend`, `auth-service`, `notify-service`, `mail-service`, `payment-service` (profil `payments`); ikuti log; status container dev |
| `make seed` | Reset database dev ke data seed (`db/scripts/reset_seed_dev.sql`) |
| `make verify-user EMAIL=... [STACK=prod]` | Lihat [`verify-user.sh`](#verify-usersh) |
| `make prod-up` | `gen-auth-keys.sh`, `check-mail-mode.sh`, `check-google-config.sh`, `check-payment-config.sh`, naikkan `db`, jalankan `08_payment_role.sh` dan `check_payment_grants.sql` (galat menghentikan target), lalu naikkan seluruh stack (profil `payments` selalu aktif) |
| `make prod-up-mailpit`, `make prod-up-smtp` | Pintasan `prod-up` dengan `MAIL_MODE` tertentu |
| `make prod-payment-check` | `check-payment-config.sh`, `check_payment_grants.sql`, dan status `payment-service`, tanpa mengubah apa pun |
| `make prod-down` | Hentikan stack prod (termasuk Mailpit dan orphan) |
| `make prod-guard` | Membuat tabel `_no_seed_guard` di DB prod agar seed/reset dev ditolak (sudah dilakukan `reset-demo.sh`) |
| `make prod-backup`, `make prod-reset`, `make prod-seed-penguji` | Lihat [`backup-prod.sh`](#backup-prodsh), [`reset-demo.sh`](#reset-demosh), [`seed-prod-penguji`](#seed-prod-penguji) |
| `make obs-up`, `make obs-down`, `make obs-url`, `make obs-check` | Observability: naikkan overlay dan role `metrics_ro`, hentikan komponennya saja, cetak alamat Grafana, jalankan `check-service-isolation.sh` dan `check-observability-leaks.sh`. `OBS_TARGET=prod` untuk stack prod |
| `make mailpit` | Cetak link Mailpit dev dan prod serta `MAIL_MODE` prod yang aktif (bawaan `mailpit`) |

`PROD_ENV` (bawaan `.env.prod`) mengganti file env prod untuk semua target `prod-*`.
