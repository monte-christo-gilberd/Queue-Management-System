import { execFileSync } from 'node:child_process';
import path from 'node:path';

import { expect, test } from '@playwright/test';

import {
  expectNoMail,
  extractLinks,
  mailpitAvailable,
  mailsTo,
  sendViaMailpitApi,
  uniqueRecipient,
  waitForMail,
} from './support';

test.describe('TS-29 Mailpit, mail-service, dan helper email (plan item 7)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    testInfo.skip(!(await mailpitAvailable(request)), 'Mailpit tidak menjawab di QMS_MAILPIT_URL (docker compose up mailpit).');
  });

  test('TC-29-01 Helper membaca email: penerima, subjek, teks, HTML, dan tautan', async ({ request }) => {
    const to = uniqueRecipient();
    await sendViaMailpitApi(request, {
      to,
      subject: 'Atur ulang password',
      text: 'Buka tautan ini: http://localhost:8080/reset-password?token=abc123.',
      html: '<p><a href="http://localhost:8080/reset-password?token=abc123&amp;x=1">Atur ulang</a></p>',
    });

    const mail = await waitForMail(request, to, { subject: 'Atur ulang' });
    expect(mail.to).toEqual([to]);
    expect(mail.from).toBe('no-reply@qms.local');
    expect(mail.text).toContain('token=abc123');
    expect(mail.html).toContain('Atur ulang');
    // Tautan dari teks (tanpa titik penutup) dan dari HTML (&amp; menjadi &), tanpa duplikat.
    expect(mail.links).toContain('http://localhost:8080/reset-password?token=abc123');
    expect(mail.links).toContain('http://localhost:8080/reset-password?token=abc123&x=1');
  });

  test('TC-29-02 Email ke penerima lain tidak terlihat, dan tidak ada email berarti daftar kosong', async ({ request }) => {
    const a = uniqueRecipient('a');
    const b = uniqueRecipient('b');
    await sendViaMailpitApi(request, { to: a, subject: 'Untuk A', text: 'halo A' });
    await waitForMail(request, a);

    expect(await mailsTo(request, b)).toHaveLength(0);
    await expectNoMail(request, b, 500);
  });

  test('TC-29-03 waitForMail gagal dengan pesan jelas bila email tidak datang', async ({ request }) => {
    const to = uniqueRecipient();
    await sendViaMailpitApi(request, { to, subject: 'Subjek lain', text: 'x' });
    await expect(waitForMail(request, to, { subject: 'Subjek yang dicari', timeoutMs: 800 })).rejects.toThrow(
      /Tidak ada email ke .* Subjek yang ada: \["Subjek lain"\]/,
    );
  });

  test('TC-29-04 extractLinks: tanpa duplikat dan tanpa tanda baca di ujung', () => {
    expect(
      extractLinks('Lihat (https://a.test/x?y=1). Lagi https://a.test/x?y=1, dan http://b.test/p;', '<a href="https://c.test/z">z</a>'),
    ).toEqual(['https://a.test/x?y=1', 'http://b.test/p', 'https://c.test/z']);
    expect(extractLinks('tanpa tautan')).toEqual([]);
  });

  // Jalur nyata mail-service -> SMTP -> Mailpit. Butuh docker pada mesin yang sama, jadi hanya jalan bila diminta:
  //   QMS_DOCKER_EXEC=1 npx playwright test ts-29
  const compose = (args: string[]) =>
    execFileSync('docker', ['compose', ...args], { cwd: path.resolve(__dirname, '../../..'), timeout: 90_000 }).toString();

  test('TC-29-05 mail-service mengirim email uji lewat SMTP dan Mailpit menerimanya', async ({ request }, testInfo) => {
    testInfo.skip(process.env.QMS_DOCKER_EXEC !== '1', 'Set QMS_DOCKER_EXEC=1 untuk menjalankan docker compose exec.');
    const to = uniqueRecipient('mailsvc');
    compose(['exec', '-T', 'mail-service', 'go', 'run', './cmd/mail-service', 'send-test', to]);

    const mail = await waitForMail(request, to, { subject: 'Email uji' });
    expect(mail.from).toBe('no-reply@qms.local');
    expect(mail.text).toContain(`Dikirim ke: ${to}`);
    expect(mail.html).toContain('<!doctype html>');
    expect(mail.links).toContain('http://localhost:8080/reset-password?token=contoh-token-uji');
  });

  test('TC-29-06 API internal mail-service: token wajib, templat dipilih pemanggil, email sampai ke Mailpit', async ({ request }, testInfo) => {
    testInfo.skip(process.env.QMS_DOCKER_EXEC !== '1', 'Set QMS_DOCKER_EXEC=1 untuk menjalankan docker compose exec.');
    const to = uniqueRecipient('api');
    const post = (token: boolean, body: object) =>
      // wget ada di image alpine; token dibaca dari env container, tidak lewat Playwright.
      compose([
        'exec', '-T', 'mail-service', 'sh', '-c',
        `wget -q -S -O - --post-data='${JSON.stringify(body)}' --header='Content-Type: application/json' ` +
          `${token ? '--header="X-Service-Token: $MAIL_SERVICE_TOKEN" ' : ''}http://127.0.0.1:8092/internal/v1/send 2>&1 || true`,
      ]);
    const body = { to, template: 'test', data: { To: to, SentAt: 'uji', Link: 'http://localhost:8080/x?y=1' } };

    expect(post(false, body)).toContain('401');
    await expect.poll(async () => (await mailsTo(request, to)).length).toBe(0);

    expect(post(true, { ...body, template: 'tidak-ada' })).toContain('400');
    expect(post(true, body)).toContain('"sent"');
    const mail = await waitForMail(request, to, { subject: 'Email uji' });
    expect(mail.links).toContain('http://localhost:8080/x?y=1');
  });

  // Peringatan: tes ini mematikan mail-service sebentar, jadi hanya jalan bila diminta dan akan menyalakannya lagi.
  test('TC-29-07 Email mati tidak mengganggu notify-service (compose tidak bergantung, tetap sehat)', async ({}, testInfo) => {
    testInfo.skip(process.env.QMS_DOCKER_EXEC !== '1', 'Set QMS_DOCKER_EXEC=1 untuk menjalankan docker compose.');
    try {
      compose(['stop', 'mail-service']);
      const state = JSON.parse(compose(['ps', '--format', 'json', 'notify-service']).trim().split('\n')[0]);
      expect(state.State).toBe('running');
      expect(state.Health).toBe('healthy');
    } finally {
      compose(['up', '-d', '--no-deps', 'mail-service']);
    }
  });
});
