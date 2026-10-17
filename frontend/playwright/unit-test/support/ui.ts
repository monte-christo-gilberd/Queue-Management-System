import { expect, type Page } from '@playwright/test';

import { AUTH_STORAGE_KEY, WEB_URL } from './env';

/** Nomor tiket di layar, mis. "A-001". */
export function ticketLabel(ticketNumber: number): string {
  return `A-${String(ticketNumber).padStart(3, '0')}`;
}

/** Tombol atau teks berdasarkan label persis. Komponen RN-web tidak selalu punya role button. */
export function action(page: Page, label: string) {
  return page.getByText(label, { exact: true });
}

/** Input berdasarkan placeholder (mengikuti yang dipakai komponen form). */
export function field(page: Page, placeholder: string) {
  return page.getByPlaceholder(placeholder, { exact: true });
}

/** Login lewat form UI. */
export async function signInThroughUi(page: Page, identifier: string, password: string) {
  await page.goto(`${WEB_URL}/`);
  await field(page, 'EMAIL ATAU USERNAME').fill(identifier);
  await field(page, 'PASSWORD').fill(password);
  await action(page, 'Login').click();
}

/**
 * Menanamkan sesi langsung ke penyimpanan browser (setup, bukan yang diuji).
 * Dipakai agar tes tidak mengulang login UI di setiap langkah.
 */
export async function signInWithToken(page: Page, token: string, displayName: string, path = '/dashboard') {
  await page.goto(`${WEB_URL}/`);
  await page.evaluate(
    ([key, value]) => window.localStorage.setItem(key, value),
    [AUTH_STORAGE_KEY, JSON.stringify({ state: { token, displayName }, version: 0 })],
  );
  await page.goto(`${WEB_URL}${path}`);
}

/** Menunggu layar login muncul (untuk memastikan user diarahkan ke sana). */
export async function expectLoginScreen(page: Page) {
  await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
}

/**
 * Input yang nilainya sama dengan `value` (form yang sudah terisi, mis. nama antrian di detail).
 * React menyinkronkan atribut `value` dari state input terkontrol.
 */
export function inputWithValue(page: Page, value: string) {
  return page.locator(`input[value="${value.replace(/"/g, '\\"')}"]`);
}
