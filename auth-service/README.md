# auth-service

Microservice login untuk QMS. Tanggung jawabnya:

- **registrasi dan login** (email atau username, bcrypt) dan menerbitkan **JWT RS256**,
- **lupa dan reset password** lewat tautan sekali pakai,
- **verifikasi email** (kirim, kirim ulang, dan verifikasi lewat tautan; setelah berhasil dikirim email konfirmasi `email-verified` di latar belakang),
- **login dengan Google** (menukar ID token Google dengan token QMS, dan menautkan akun lewat konfirmasi password),
- menjadi **satu-satunya pemegang kunci privat** penanda token (`AUTH_JWT_PRIVATE_KEY_FILE`).

Backend **tidak memanggil** service ini. Mereka memverifikasi token secara lokal dengan kunci publik
(`keys/auth-public.pem`) dan membaca pencabutan sesi serta status verifikasi langsung dari database. Bila auth-service
mati, user yang sudah login tetap bisa memakai aplikasi; yang gagal hanya login baru, registrasi, reset password, dan
verifikasi email (nginx menjawab 503 `AUTH_UNAVAILABLE`).

## Menjalankan

Kunci dibuat sekali (otomatis oleh `make up` dan `make prod-up`):

```bash
scripts/gen-auth-keys.sh        # dari akar queue-management-system/
```

Lewat compose (service `auth-service`, port internal 8094, tidak dipetakan ke host):

```bash
docker compose up -d --build auth-service
curl -s http://localhost:8080/api/v1/users/login -H 'Content-Type: application/json' \
  -d '{"identifier":"budi","password":"rahasia123"}'      # lewat nginx
```

Tanpa Docker: `go run ./cmd/auth-service` (membaca `../.env`; butuh `DATABASE_URL`, `PUBLIC_APP_URL`,
`AUTH_JWT_PRIVATE_KEY_FILE`). Tes: `go test ./...`.

## Endpoint

Path publik sama dengan yang dulu dilayani backend (nginx meneruskannya ke service ini). Bentuk respons:
sukses `{"data": ...}`, gagal `{"error": {"code", "message"}}`.

| Method | Path | Keterangan |
|---|---|---|
| GET | `/health` | Liveness (`{"status":"ok"}`), tidak menyentuh database |
| POST | `/api/v1/users/register` | `{email, username, password}` → `{data:{token}}`. 400 `WEAK_PASSWORD`, 409 `CONFLICT` |
| POST | `/api/v1/users/login` | `{identifier, password}` (`email` diterima sebagai alias) → `{data:{token}}`. Semua kegagalan 401 `UNAUTHORIZED` |
| POST | `/api/v1/users/google` | `{id_token}` → `{data:{token}}`, atau 409 `GOOGLE_LINK_REQUIRED` + `error.link_token`. 401/403/503 lihat kontrak API. Hanya bila `GOOGLE_CLIENT_IDS` terisi |
| POST | `/api/v1/users/google/link` | `{link_token, password}` → `{data:{token}}`. 400 `INVALID_TOKEN`, 401 `UNAUTHORIZED`, 409 `CONFLICT` |
| POST | `/api/v1/users/forgot-password` | Selalu 202 untuk email berformat sah (tidak ada enumerasi akun) |
| POST | `/api/v1/users/reset-password` | `{token, password}`. 400 `INVALID_TOKEN` / `WEAK_PASSWORD` |
| POST | `/api/v1/users/verify-email` | `{token}`. 400 `INVALID_TOKEN` |
| POST | `/api/v1/users/me/verify-email/resend` | Butuh login. 202, atau 429 bila lewat 3 per jam |

Forgot/reset/verify hanya terpasang bila `MAIL_SERVICE_URL` terisi (selain itu 404). Profil, avatar, preferensi, dan tiket
tetap di backend.

## Token

```
header : {"alg":"RS256","typ":"JWT","kid":"qms-1"}
payload: {"sub":"<user id>","iat":<detik>,"exp":<iat + 24 jam>}
```

`iat` dipakai pencabutan sesi: token yang terbit sebelum `users.password_changed_at` ditolak. Verifikasi hanya menerima
RS256 (HS256 dan `alg=none` ditolak).

**Link token** (penautan akun Google): JWT RS256 5 menit dengan `purpose: "google_link"`, `gid`, dan `jti` acak. Ditandatangani
kunci yang sama dengan token login, tetapi **bukan token login**: `purpose` membuatnya ditolak oleh `auth-service` dan backend.
Sekali pakai, dan hangus setelah 5 kali salah password (penghitung di memori; satu instance).

## Login Google

`internal/google` memeriksa ID token sendiri (tanpa `google.golang.org/api`): tanda tangan RS256 dengan JWKS Google
(di-cache sesuai `Cache-Control`, `kid` tak dikenal paling cepat diambil ulang tiap 1 menit, kunci kedaluwarsa tidak dipakai
bila Google mati), `iss` Google, `aud` salah satu `GOOGLE_CLIENT_IDS`, `exp`/`iat` dengan toleransi 60 detik, dan
`email_verified`. Keputusan di `service/google_service.go`: akun tertaut (cocok lewat `google_id`) → token; email belum ada → akun baru
tanpa password dan terverifikasi; email sudah ada dengan password (huruf besar/kecil diabaikan) → 409 dan **tidak** ditautkan
otomatis.

## Konfigurasi

| Variabel | Wajib | Keterangan |
|---|---|---|
| `DATABASE_URL` (atau `AUTH_DATABASE_URL`) | ya | Koneksi dengan role `auth_rw` |
| `PUBLIC_APP_URL` | ya | Alamat publik untuk tautan di email. Produksi: wajib https |
| `AUTH_JWT_PRIVATE_KEY_FILE` | ya | Kunci privat RSA (PEM, PKCS#8 atau PKCS#1) |
| `AUTH_JWT_KID` | tidak | Header `kid` (bawaan `qms-1`) |
| `MAIL_SERVICE_URL`, `MAIL_SERVICE_TOKEN` | pasangan | Keduanya atau tidak sama sekali. Produksi: token minimal 32 karakter |
| `EMAIL_VERIFICATION_REQUIRED` | tidak | Kosong = ikut ada-tidaknya mail-service. **Harus sama dengan backend** |
| `GOOGLE_CLIENT_IDS` | tidak | Client ID OAuth yang sah (web, Android, iOS) untuk klaim `aud`, dipisah koma. Kosong = login Google mati (404). Lihat [setup-google-login.md](../../documentation/docs/setup-google-login.md) |
| `GOOGLE_CERTS_URL` | tidak | Alamat kunci publik Google; hanya untuk tes lokal. Produksi: wajib https |
| `PORT` / `AUTH_PORT` | tidak | Bawaan `8094` |
| `TRUSTED_PROXIES` | tidak | CIDR proxy yang boleh menentukan IP klien |
| `SHUTDOWN_TIMEOUT` | tidak | Detik, bawaan 8 |
| `APP_ENV` | tidak | `production` mengaktifkan pemeriksaan keamanan |

## Database

Role `auth_rw` (`db/migrations/06_auth_role.sh`): baca kolom login di `users`, tulis `password_hash`,
`password_changed_at`, `email_verified_at`, `google_id`, dan `INSERT` user baru; `user_action_tokens` baca, tambah, dan
ubah `used_at` (tanpa `DELETE`). Tidak ada akses ke antrian, tiket, atau pembayaran. Yang disimpan untuk token sekali
pakai hanya hash SHA-256, tidak pernah token mentah.

## Struktur

```
cmd/auth-service/    main: main() memanggil run(); konfigurasi → database → repository → service → handler → router, shutdown rapi
internal/apperror/   galat sentinel yang dipetakan handler ke kode HTTP
internal/config/     env, kunci privat, validasi produksi, TRUSTED_PROXIES
internal/database/   koneksi PostgreSQL (Connect)
internal/dto/        body request dan isi respons JSON
internal/google/     verifikasi ID token Google (JWKS, iss, aud, exp, email_verified)
internal/handler/    Gin: 8 handler akun (AuthHandler), respondData/respondError
internal/httpserver/ http.Server dengan batas waktu
internal/mailclient/ klien HTTP ke mail-service
internal/middleware/ Auth: pemeriksaan JWT RS256 dan pencabutan sesi
internal/model/      User dan GoogleIdentity
internal/repository/ PostgreSQL: UserRepository dan TokenRepository
internal/router/     AppRouter, NewAppRouter + WithEmail/WithGoogle, SetupRouter, rute per fitur
internal/service/    Service (register, login, Authenticate, reset, verifikasi, Google) dan TokenIssuer (RS256)
internal/validator/  aturan no_html_tags
internal/test/       tes (fake di memori di testutil, sqlmock untuk SQL)
```
