import { execFileSync } from 'node:child_process';
import path from 'node:path';

import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  createQueueViaApi,
  expectNoMail,
  joinQueueViaApi,
  listTicketsViaApi,
  mailpitAvailable,
  mailsTo,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  waitForMail,
} from './support';

const pause = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function newQueue(request: APIRequestContext, prefix = 'Mail') {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName(prefix));
  return { owner: session.token, queue };
}

async function ticketIdOf(request: APIRequestContext, owner: string, queueId: number, number: number) {
  const res = await listTicketsViaApi(request, owner, queueId);
  const t = ((await res.json()).data.tickets as { id: number; ticket_number: number }[]).find((x) => x.ticket_number === number);
  if (!t) throw new Error(`tiket ${number} tidak ada`);
  return t.id;
}

async function setEmailPref(request: APIRequestContext, token: string, enabled: boolean) {
  const res = await request.put(`${API_URL}/api/v1/users/me/preferences`, {
    headers: auth(token),
    data: { email_notifications: enabled },
  });
  expect(res.status(), await res.text()).toBe(200);
}

test.describe('TS-30 Email notifikasi tiket lewat outbox (plan item 8)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    testInfo.skip((await subscribedOwnerSession(request)) === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
    testInfo.skip(!(await mailpitAvailable(request)), 'Mailpit tidak menjawab di QMS_MAILPIT_URL.');
  });

  test('TC-30-01 Join mengirim satu email berbahasa Indonesia berisi nomor dan tautan, join ulang tidak menggandakan', async ({ request }) => {
    const { queue } = await newQueue(request, 'Join');
    const a = await registerAccount(request, 'mja');
    const b = await registerAccount(request, 'mjb');
    await joinQueueViaApi(request, a.token, queue.id);
    await joinQueueViaApi(request, b.token, queue.id);

    const mail = await waitForMail(request, a.email, { subject: 'Anda masuk antrian' });
    expect(mail.subject).toBe(`Anda masuk antrian ${queue.queue_name}: nomor A-001`);
    expect(mail.text).toContain(`Halo ${a.username}`);
    expect(mail.links.some((l) => new RegExp(`^https?://[^/]+/ticket/${queue.id}$`).test(l))).toBe(true);
    expect(mail.html).toContain('A-001');

    // B hanya menerima emailnya sendiri (nomor A-002).
    const forB = await waitForMail(request, b.email, { subject: 'Anda masuk antrian' });
    expect(forB.subject).toContain('A-002');
    expect(await mailsTo(request, b.email)).toHaveLength(1);

    // Join ulang mengembalikan tiket yang sama dan tidak menulis outbox lagi.
    await joinQueueViaApi(request, a.token, queue.id);
    await pause(1500);
    expect(await mailsTo(request, a.email)).toHaveLength(1);
  });

  test('TC-30-02 Customer membatalkan: email "dibatalkan"', async ({ request }) => {
    const { queue } = await newQueue(request, 'Cancel');
    const a = await registerAccount(request, 'mca');
    await joinQueueViaApi(request, a.token, queue.id);
    await waitForMail(request, a.email, { subject: 'Anda masuk antrian' });

    const res = await request.post(`${API_URL}/api/v1/queues/${queue.id}/cancel`, { headers: auth(a.token) });
    expect(res.status()).toBe(200);

    const mail = await waitForMail(request, a.email, { subject: 'dibatalkan' });
    expect(mail.text).toContain('telah dibatalkan');
    expect(mail.subject).toContain('A-001');
  });

  test('TC-30-03 Owner menghapus nomor: email ke pemilik nomor saja', async ({ request }) => {
    const { owner, queue } = await newQueue(request, 'Remove');
    const a = await registerAccount(request, 'mra');
    const b = await registerAccount(request, 'mrb');
    await joinQueueViaApi(request, a.token, queue.id);
    await joinQueueViaApi(request, b.token, queue.id);

    const id = await ticketIdOf(request, owner, queue.id, 1);
    const res = await request.post(`${API_URL}/api/v1/queues/${queue.id}/tickets/${id}/remove`, { headers: auth(owner) });
    expect(res.status(), await res.text()).toBe(200);

    const mail = await waitForMail(request, a.email, { subject: 'dihapus oleh penyelenggara' });
    expect(mail.text).toContain('menghapus nomor Anda');
    // B tidak ikut menerima, hanya email join miliknya.
    await waitForMail(request, b.email, { subject: 'Anda masuk antrian' });
    expect((await mailsTo(request, b.email)).map((m) => m.subject).join('|')).not.toContain('dihapus');
  });

  test('TC-30-04 Selesai dilayani: email hanya untuk nomor yang selesai, bukan yang baru dipanggil', async ({ request }) => {
    const { owner, queue } = await newQueue(request, 'Done');
    const a = await registerAccount(request, 'mda');
    const b = await registerAccount(request, 'mdb');
    await joinQueueViaApi(request, a.token, queue.id);
    await joinQueueViaApi(request, b.token, queue.id);

    expect((await nextViaApi(request, owner, queue.id)).ok()).toBeTruthy(); // A dilayani
    await pause(1000);
    expect((await mailsTo(request, a.email)).map((m) => m.subject).join('|')).not.toContain('Selesai');

    expect((await nextViaApi(request, owner, queue.id)).ok()).toBeTruthy(); // A selesai, B dilayani
    const mail = await waitForMail(request, a.email, { subject: 'Selesai dilayani' });
    expect(mail.subject).toBe(`Selesai dilayani: nomor A-001 di ${queue.queue_name}`);
    expect((await mailsTo(request, b.email)).map((m) => m.subject).join('|')).not.toContain('Selesai');
  });

  test('TC-30-05 Sakelar mati: tidak ada email untuk kejadian berikutnya, menyala lagi: email datang', async ({ request }) => {
    const { queue } = await newQueue(request, 'Pref');
    const a = await registerAccount(request, 'mpa');
    await setEmailPref(request, a.token, false);
    await joinQueueViaApi(request, a.token, queue.id);
    await expectNoMail(request, a.email, 2500);

    await setEmailPref(request, a.token, true);
    const res = await request.post(`${API_URL}/api/v1/queues/${queue.id}/cancel`, { headers: auth(a.token) });
    expect(res.status()).toBe(200);
    const mail = await waitForMail(request, a.email, { subject: 'dibatalkan' });
    expect(mail.subject).toContain('A-001');
    // Email join yang terlewat saat sakelar mati tidak dikirim belakangan.
    expect((await mailsTo(request, a.email)).map((m) => m.subject).join('|')).not.toContain('Anda masuk');
  });

  test('TC-30-06 API sakelar: default menyala, nilai salah 400, tanpa token 401', async ({ request }) => {
    const a = await registerAccount(request, 'mpb');
    const me = await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(a.token) });
    expect((await me.json()).data.email_notifications).toBe(true);

    for (const data of [{}, { email_notifications: null }, { email_notifications: 'ya' }]) {
      const res = await request.put(`${API_URL}/api/v1/users/me/preferences`, { headers: auth(a.token), data });
      expect(res.status(), JSON.stringify(data)).toBe(400);
    }
    const noAuth = await request.put(`${API_URL}/api/v1/users/me/preferences`, { data: { email_notifications: false } });
    expect(noAuth.status()).toBe(401);

    await setEmailPref(request, a.token, false);
    const after = await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(a.token) });
    expect((await after.json()).data.email_notifications).toBe(false);
  });

  test('TC-30-07 UI profil: sakelar notifikasi email tersimpan di server dan bertahan setelah reload', async ({ page, request }) => {
    const a = await registerAccount(request, 'mpu');
    await signInWithToken(page, a.token, a.username, '/profile');
    const toggle = page.getByRole('switch', { name: 'Notifikasi email' });
    await expect(toggle).toBeChecked();

    await toggle.click();
    await expect(toggle).not.toBeChecked();
    await expect
      .poll(async () => (await (await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(a.token) })).json()).data.email_notifications)
      .toBe(false);

    await page.reload();
    await expect(page.getByRole('switch', { name: 'Notifikasi email' })).not.toBeChecked();
    await expect(action(page, 'Notifikasi email')).toBeVisible();
  });

  // Ketahanan memakai docker compose pada mesin yang sama, jadi hanya jalan bila diminta:
  //   QMS_DOCKER_EXEC=1 npx playwright test ts-30
  const compose = (args: string[]) =>
    execFileSync('docker', ['compose', ...args], { cwd: path.resolve(__dirname, '../../..'), timeout: 120_000 }).toString();

  test('TC-30-08 mail-service mati: join tetap berhasil, email terkirim tepat sekali setelah hidup lagi', async ({ request }, testInfo) => {
    testInfo.skip(process.env.QMS_DOCKER_EXEC !== '1', 'Set QMS_DOCKER_EXEC=1 untuk menjalankan docker compose.');
    testInfo.setTimeout(120_000);
    const { queue } = await newQueue(request, 'Down');
    const a = await registerAccount(request, 'mxa');
    compose(['stop', 'mail-service']);
    try {
      await joinQueueViaApi(request, a.token, queue.id); // tidak boleh gagal karena email mati
      await expectNoMail(request, a.email, 2500);
    } finally {
      compose(['start', 'mail-service']);
    }
    const mail = await waitForMail(request, a.email, { subject: 'Anda masuk antrian', timeoutMs: 40_000 });
    expect(mail.subject).toContain('A-001');
    await pause(1500);
    expect(await mailsTo(request, a.email)).toHaveLength(1);
  });

  test('TC-30-09 Mailpit mati: aksi tetap berhasil dan email dicoba ulang sampai Mailpit hidup', async ({ request }, testInfo) => {
    testInfo.skip(process.env.QMS_DOCKER_EXEC !== '1', 'Set QMS_DOCKER_EXEC=1 untuk menjalankan docker compose.');
    testInfo.setTimeout(180_000);
    const { queue } = await newQueue(request, 'SmtpDown');
    const a = await registerAccount(request, 'mxb');
    compose(['stop', 'mailpit']);
    try {
      await joinQueueViaApi(request, a.token, queue.id);
      await pause(4000);
    } finally {
      compose(['start', 'mailpit']);
    }
    // Mailpit butuh beberapa detik untuk siap setelah start; baru sesudah itu boleh membaca kotak masuknya.
    await expect.poll(() => mailpitAvailable(request), { timeout: 30_000 }).toBe(true);
    // Percobaan ulang memakai jeda 30 detik pertama: tunggu sampai satu siklus.
    const mail = await waitForMail(request, a.email, { subject: 'Anda masuk antrian', timeoutMs: 120_000 });
    expect(mail.subject).toContain('A-001');
  });
});
