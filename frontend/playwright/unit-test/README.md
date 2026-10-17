# E2E Queue Management System (Playwright)

Tes ini menguji **behaviour yang diharapkan** dari `Capstone-Project/documentation/docs/product-overview.md`, bukan detail implementasi. Setiap test case punya ID `TC-NN-MM` dan sumber di dokumen test case (`test-cases/queue-management/`).

## Prasyarat

1. Database `qms_db` berjalan, dengan schema dari `migrations/01_Schema.sql`.
2. Backend: `cd backend && go run ./cmd/server` (port 8080).
3. Frontend: `cd frontend && npx expo start --web` (port 8081).
4. Akun owner berlangganan (belum ada endpoint untuk ini). Buat akun lewat register, lalu di database:
   ```sql
   INSERT INTO owners (user_id, subscription_expires_at)
   VALUES (<id user>, now() + interval '30 days');
   ```
5. Dependensi: `npm install` di `frontend/` (sudah memuat `@playwright/test`). Browser Chromium sudah tersedia di cache; bila belum, `npx playwright install chromium`.

## Variabel lingkungan

| Variabel | Default | Keterangan |
|---|---|---|
| `QMS_WEB_URL` | `http://localhost:8081` | Alamat frontend |
| `QMS_API_URL` | `http://localhost:8080` | Alamat backend |
| `QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD` | — | Owner berlangganan. Tanpa ini, tes owner/customer di-skip |
| `QMS_EXPIRED_OWNER_EMAIL` / `QMS_EXPIRED_OWNER_PASSWORD` | — | Owner dengan langganan kedaluwarsa (opsional, untuk TC-04-03) |

## Menjalankan

Dari folder `frontend/`:

```bash
QMS_OWNER_EMAIL=owner@example.com QMS_OWNER_PASSWORD=... npm run e2e
npx playwright test ts-01 --headed        # satu area, tampilkan browser
npm run e2e:report                        # buka laporan HTML
```

`testDir` di `playwright.config.ts` adalah `playwright/unit-test`. Hasil (trace, screenshot) masuk ke `playwright/.results`, laporan ke `playwright/.report` (keduanya di-ignore git).

## Catatan desain

- **Berurutan (`workers: 1`)**: beberapa tes berbagi satu akun owner. Dashboard menampilkan antrian aktif terbaru, jadi tes tidak boleh berjalan bersamaan.
- **Setup lewat API**: akun, antrian, dan join disiapkan lewat API (`support/api.ts`), supaya tes yang diuji hanya langkah yang relevan.
- **Sesi lewat penyimpanan browser**: `signInWithToken` menanamkan token ke `localStorage` (`qms-auth`) untuk setup. Alur login yang sebenarnya diuji di TS-01.
- **Backlog**: `ts-09` berstatus `fixme`. Ubah ke tes biasa saat fitur selesai.
- **Real-time**: TC-07-04 memutus stream SSE dengan `page.route` lalu membukanya lagi.

## Status

Terdaftar 68 tes (`npx playwright test --list`). Hasil run pertama terhadap aplikasi yang berjalan dicatat di bagian bawah file ini.
