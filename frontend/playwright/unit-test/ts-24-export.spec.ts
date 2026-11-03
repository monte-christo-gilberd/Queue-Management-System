import { readFileSync } from 'node:fs';

import { expect, test } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  closedQueueWithAllOutcomes,
  createQueueViaApi,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

const HEADER = 'Nomor,Nama,Email,Hasil,Bergabung (UTC),Dipanggil (UTC),Selesai (UTC),Dibatalkan (UTC),Jumlah Dilewati,Counter,Catatan';

/** Baris CSV tanpa BOM. Isi sel dalam spec ini tidak memuat koma atau baris baru, jadi cukup dipisah sederhana. */
function csvLines(text: string): string[] {
  expect(text.charCodeAt(0)).toBe(0xfeff); // BOM UTF-8
  return text.slice(1).split('\r\n').filter((l) => l !== '');
}

test.describe('TS-24 Ekspor riwayat antrian (CSV dan Excel)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-24-01 CSV memuat judul, enam nomor dengan hasil akhir, dan catatan', async ({ request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request);

    const res = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}/export?format=csv`, { headers: auth(owner.token) });
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('text/csv');
    expect(res.headers()['content-disposition']).toMatch(new RegExp(`^attachment; filename="riwayat-[a-z0-9-]+-${queue.id}\\.csv"$`));
    expect(res.headers()['cache-control']).toBe('no-store');

    const lines = csvLines(await res.text());
    expect(lines[0]).toBe(HEADER);
    expect(lines).toHaveLength(7);
    const outcomes = lines.slice(1).map((l) => l.split(',')[3]);
    expect(outcomes).toEqual(['Dilewati', 'Selesai', 'Selesai', 'Dibatalkan', 'Dihapus', 'Belum dilayani']);
    expect(lines[1].startsWith('A-001,')).toBe(true);
    expect(lines[3]).toContain('minta struk');
    expect(lines[2]).not.toContain('minta struk');
  });

  test('TC-24-02 Catatan berawalan = diberi pengaman formula injection', async ({ request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request, { note: '=HYPERLINK("http://x")' });

    const res = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}/export?format=csv`, { headers: auth(owner.token) });
    const text = await res.text();
    expect(text).toContain(`'=HYPERLINK`);
    expect(text).not.toMatch(/(^|,|")=HYPERLINK/m);
  });

  test('TC-24-03 Excel berupa berkas xlsx yang valid', async ({ request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request);

    const res = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}/export?format=xlsx`, { headers: auth(owner.token) });
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toBe('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
    expect(res.headers()['content-disposition']).toMatch(/filename="riwayat-[a-z0-9-]+-\d+\.xlsx"/);
    const body = await res.body();
    expect(body.subarray(0, 4).toString('hex')).toBe('504b0304'); // tanda tangan zip "PK\x03\x04"
    expect(body.length).toBeGreaterThan(1000);
  });

  test('TC-24-04 Galat: format salah 400, bukan owner 404, antrian aktif 404, tanpa login 401', async ({ request }) => {
    const { owner, queue, customers } = await closedQueueWithAllOutcomes(request);
    const url = `${API_URL}/api/v1/history/queues/${queue.id}/export`;

    for (const query of ['', '?format=', '?format=pdf', '?format=CSV']) {
      const bad = await request.get(`${url}${query}`, { headers: auth(owner.token) });
      expect(bad.status(), query).toBe(400);
      expect(bad.headers()['content-disposition'], query).toBeUndefined();
    }
    expect((await request.get(`${url}?format=csv`, { headers: auth(customers[0].token) })).status()).toBe(404);
    expect((await request.get(`${url}?format=csv`)).status()).toBe(401);

    const active = await createQueueViaApi(request, owner.token, uniqueName('Aktif'));
    const activeRes = await request.get(`${API_URL}/api/v1/history/queues/${active.id}/export?format=csv`, { headers: auth(owner.token) });
    expect(activeRes.status()).toBe(404);
  });

  test('TC-24-05 UI: tombol Ekspor CSV dan Ekspor Excel mengunduh berkas', async ({ page, request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request);
    await signInWithToken(page, owner.token, owner.displayName, `/history/${queue.id}`);
    await expect(page.getByText('Semua Nomor Antrian')).toBeVisible();

    const [csv] = await Promise.all([page.waitForEvent('download'), action(page, 'Ekspor CSV').click()]);
    expect(csv.suggestedFilename()).toMatch(new RegExp(`^riwayat-[a-z0-9-]+-${queue.id}\\.csv$`));
    expect(readFileSync(await csv.path(), 'utf8')).toContain('Nomor,Nama,Email,Hasil');

    const [xlsx] = await Promise.all([page.waitForEvent('download'), action(page, 'Ekspor Excel').click()]);
    expect(xlsx.suggestedFilename()).toMatch(new RegExp(`^riwayat-[a-z0-9-]+-${queue.id}\\.xlsx$`));
    expect(readFileSync(await xlsx.path()).subarray(0, 2).toString()).toBe('PK');
  });
});
