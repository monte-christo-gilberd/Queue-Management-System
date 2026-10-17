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
5. Dependensi Playwright di folder ini: `npm install` lalu `npx playwright install chromium`.

## Variabel lingkungan

| Variabel | Default | Keterangan |
|---|---|---|
| `QMS_WEB_URL` | `http://localhost:8081` | Alamat frontend |
| `QMS_API_URL` | `http://localhost:8080` | Alamat backend |
| `QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD` | — | Owner berlangganan. Tanpa ini, tes owner/customer di-skip |
| `QMS_EXPIRED_OWNER_EMAIL` / `QMS_EXPIRED_OWNER_PASSWORD` | — | Owner dengan langganan kedaluwarsa (opsional, untuk TC-04-03) |

## Menjalankan

```bash
cd playwright-template
QMS_OWNER_EMAIL=owner@example.com QMS_OWNER_PASSWORD=... npm test
```

Hanya folder ini yang dijalankan (`testDir` di `playwright.config.ts`). Tes POS di `tests/ts-*.spec.ts` tidak ikut.

## Catatan desain

- **Berurutan (`workers: 1`)**: beberapa tes berbagi satu akun owner. Dashboard menampilkan antrian aktif terbaru, jadi tes tidak boleh berjalan bersamaan.
- **Setup lewat API**: akun, antrian, dan join disiapkan lewat API (`support/api.ts`), supaya tes yang diuji hanya langkah yang relevan.
- **Sesi lewat penyimpanan browser**: `signInWithToken` menanamkan token ke `localStorage` (`qms-auth`) untuk setup. Alur login yang sebenarnya diuji di TS-01.
- **Backlog**: `ts-09` berstatus `fixme`. Ubah ke tes biasa saat fitur selesai.
- **Real-time**: TC-07-04 memutus stream SSE dengan `page.route` lalu membukanya lagi.

## Status

Tes ini sudah dikompilasi dengan TypeScript strict dan terdaftar (`playwright test --list`: 68 tes). **Belum dijalankan terhadap aplikasi yang sedang berjalan.** Jalankan dulu TS-01 untuk memastikan selektor sesuai dengan tampilan sebenarnya, lalu lanjut ke area lain.
