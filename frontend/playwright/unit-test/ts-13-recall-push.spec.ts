import { expect, test } from '@playwright/test';

import {
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  recallViaApi,
  registerAccount,
  subscribedOwnerSession,
  uniqueName,
} from './support';

const API = process.env.QMS_API_URL ?? 'http://localhost:8080';

test.describe('TS-13 Panggil ulang dan Web Push', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('recall: belum ada yang dilayani, terlalu cepat, lalu sekali saja', async ({ request }) => {
    const session = (await subscribedOwnerSession(request))!;
    const queue = await createQueueViaApi(request, session.token, uniqueName('Recall'));
    const a = await registerAccount(request, 'recall');
    await joinQueueViaApi(request, a.token, queue.id);

    let res = await recallViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('NOTHING_TO_RECALL');

    await nextViaApi(request, session.token, queue.id);
    res = await recallViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('RECALL_TOO_SOON');

    await new Promise((r) => setTimeout(r, 3200));
    res = await recallViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(200);

    res = await recallViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('ALREADY_RECALLED');

    const denied = await recallViaApi(request, a.token, queue.id);
    expect(denied.status()).toBe(404);
  });

  test('push: public key, subscribe, unsubscribe, validasi endpoint', async ({ request }) => {
    const u = await registerAccount(request, 'push');
    const h = { Authorization: `Bearer ${u.token}` };

    const key = await request.get(`${API}/api/v1/push/public-key`, { headers: h });
    expect(key.status()).toBe(200);
    expect((await key.json()).data.public_key).toBeTruthy();

    const body = { endpoint: `https://push.example.com/${Date.now()}`, keys: { p256dh: 'a', auth: 'b' } };
    expect((await request.post(`${API}/api/v1/push/subscribe`, { headers: h, data: body })).status()).toBe(200);
    expect(
      (await request.post(`${API}/api/v1/push/subscribe`, { headers: h, data: { ...body, endpoint: 'http://x.y/z' } })).status(),
    ).toBe(400);
    expect(
      (await request.post(`${API}/api/v1/push/unsubscribe`, { headers: h, data: { endpoint: body.endpoint } })).status(),
    ).toBe(200);
    expect((await request.get(`${API}/api/v1/push/public-key`)).status()).toBe(401);
  });
});
