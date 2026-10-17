import { defineConfig, devices } from '@playwright/test';

/**
 * E2E frontend (Expo web) + backend Go. Jalankan dari folder `frontend/`.
 *
 * Prasyarat: backend `go run ./cmd/server` (port 8080) dan `npx expo start --web` (port 8081).
 * Variabel lingkungan: lihat playwright/unit-test/README.md.
 */
export default defineConfig({
  testDir: './playwright/unit-test',
  // Beberapa tes berbagi satu akun owner berlangganan; antrean dashboard memakai antrian aktif terbaru.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  outputDir: './playwright/.results',
  reporter: [['list'], ['html', { outputFolder: './playwright/.report', open: 'never' }]],
  use: {
    baseURL: process.env.QMS_WEB_URL ?? 'http://localhost:8081',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    viewport: { width: 1280, height: 800 },
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
