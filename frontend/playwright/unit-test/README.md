# E2E Queue Management System (Playwright)

Tes ini menguji **behaviour yang diharapkan** dari `Capstone-Project/documentation/docs/product-overview.md`, bukan detail implementasi. Setiap test case punya ID `TC-NN-MM` dan sumber di dokumen test case (`test-cases/queue-management/`).

## Prasyarat

1. Jalankan stack: `docker compose up -d`. Semua dijangkau lewat nginx di `http://localhost:8080`.
2. Seed ulang sebelum tiap run (owner1/owner2, user1..user10, password `password`):
   ```bash
   docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql
   ```
5. Dependensi: `npm install` di `frontend/` (sudah memuat `@playwright/test`). Browser Chromium sudah tersedia di cache; bila belum, `npx playwright install chromium`.

## Variabel lingkungan

| Variabel | Default | Keterangan |
|---|---|---|
| `QMS_WEB_URL` | `http://localhost:8080` | Alamat frontend (lewat nginx) |
| `QMS_API_URL` | `http://localhost:8080` | Alamat backend |
| `QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD` | — | Owner berlangganan. Tanpa ini, tes owner/customer di-skip |
| `QMS_EXPIRED_OWNER_EMAIL` / `QMS_EXPIRED_OWNER_PASSWORD` | `owner3@test.com` / `password` | Owner kedaluwarsa dari seed (untuk TC-04-03) |

## Menjalankan

Dari folder `frontend/`:

```bash
QMS_WEB_URL=http://localhost:8080 QMS_API_URL=http://localhost:8080 \
QMS_OWNER_EMAIL=owner1@test.com QMS_OWNER_PASSWORD=password npm run e2e
npx playwright test ts-01 --headed        # satu area, tampilkan browser
npm run e2e:report                        # buka laporan HTML
```

`testDir` di `playwright.config.ts` adalah `playwright/unit-test`. Hasil (trace, screenshot) masuk ke `playwright/.results`, laporan ke `playwright/.report` (keduanya di-ignore git).

## Catatan desain

- **Berurutan (`workers: 1`)**: beberapa tes berbagi satu akun owner. Dashboard menampilkan antrian aktif terbaru, jadi tes tidak boleh berjalan bersamaan.
- **Setup lewat API**: akun, antrian, dan join disiapkan lewat API (`support/api.ts`), supaya tes yang diuji hanya langkah yang relevan.
- **Sesi lewat penyimpanan browser**: `signInWithToken` menanamkan token ke `localStorage` (`qms-auth`) untuk setup. Alur login yang sebenarnya diuji di TS-01.
- **Backlog**: `ts-09` berstatus `fixme`. Ubah ke tes biasa saat fitur selesai.
- **Real-time**: TC-07-04 memutus stream SSE dengan `page.route` lalu membukanya lagi. TC-07-05 memblokir SSE dan memajukan jam (`page.clock.fastForward`) untuk menguji resinkron.
- **Join dua kali**: oleh user yang sama mengembalikan 200 dengan tiket yang sama (TC-05-04).
- **TC-04-03**: memakai `owner3` dari seed (langganan kedaluwarsa, satu antrian aktif).

## Login Google (TS-34)

`ts-34-google-login.spec.ts` menguji sisi frontend login Google tanpa Google asli: popup `accounts.google.com` dicegat dan dijawab
seperti Google mengalihkan balik (dengan `state` yang sah), dan `POST /users/google` serta `/users/google/link` disetubkan. Dua tes
(TC-34-12, TC-34-13) memanggil backend sungguhan. Prasyarat: bundle dibangun dengan `EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID` dan aplikasi
dibuka lewat `http://localhost:8080` (bukan alamat IP; WebCrypto hanya ada di secure origin). Tanpa itu tes tampilan di-skip.

```bash
QMS_WEB_URL=http://localhost:8080 QMS_API_URL=http://localhost:8080 npx playwright test ts-34
```

## Pesan konfirmasi password (TS-35)

`ts-35-confirm-password.spec.ts` menguji kapan pesan "Password tidak sama" tampil di halaman Daftar dan Reset Password: sebagai
teks di bawah field konfirmasi (bukan toast), tidak saat pengguna masih mengetik, baru setelah field ditinggalkan atau tombol
kirim ditekan, dan hilang saat isinya sudah sama. Tes tidak butuh Mailpit dan tidak membuat akun. `TC-01-08` memeriksa sisi yang sama
dari alur Daftar.

```bash
QMS_WEB_URL=http://localhost:8080 QMS_API_URL=http://localhost:8080 npx playwright test ts-35
```

## Status

Run dari seed bersih: 76 lulus, 0 gagal, 16 di-skip (backlog), TC-04-03 ikut berjalan.
