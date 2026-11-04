import { expect, type APIRequestContext } from '@playwright/test';

/** Alamat UI/API Mailpit (dev). Hanya terikat ke localhost mesin ini, jadi tes harus jalan di mesin yang sama. */
export const MAILPIT_URL = process.env.QMS_MAILPIT_URL ?? 'http://localhost:8025';

export type Mail = {
  id: string;
  to: string[];
  from: string;
  subject: string;
  text: string;
  html: string;
  /** Semua tautan http(s) di badan teks dan HTML, tanpa duplikat, urut kemunculan. */
  links: string[];
};

type ApiAddress = { Address: string };
type ApiSummary = { ID: string; To: ApiAddress[]; Subject: string; From: ApiAddress };
type ApiMessage = ApiSummary & { Text: string; HTML: string };

/** true bila Mailpit menjawab; tes email di-skip bila tidak (mis. stack tanpa Mailpit). */
export async function mailpitAvailable(request: APIRequestContext): Promise<boolean> {
  try {
    const res = await request.get(`${MAILPIT_URL}/api/v1/info`, { timeout: 3000 });
    return res.ok();
  } catch {
    return false;
  }
}

/** Alamat penerima unik per tes, supaya tes tidak saling melihat email (kotak surat dibagi semua tes). */
export function uniqueRecipient(prefix = 'mail'): string {
  return `${prefix}-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}@qms-test.local`;
}

/** Tautan http(s) dari teks, tanpa tanda baca penutup di ujung. Fungsi murni. */
export function extractLinks(...bodies: string[]): string[] {
  const found: string[] = [];
  for (const body of bodies) {
    for (const m of body.matchAll(/https?:\/\/[^\s"'<>]+/g)) {
      const link = m[0].replace(/[).,;:!?]+$/, '').replace(/&amp;/g, '&');
      if (!found.includes(link)) found.push(link);
    }
  }
  return found;
}

async function fetchMessage(request: APIRequestContext, id: string): Promise<Mail> {
  const res = await request.get(`${MAILPIT_URL}/api/v1/message/${id}`);
  expect(res.ok(), `GET message ${id}`).toBeTruthy();
  const m = (await res.json()) as ApiMessage;
  return {
    id: m.ID,
    to: m.To.map((a) => a.Address),
    from: m.From.Address,
    subject: m.Subject,
    text: m.Text,
    html: m.HTML,
    links: extractLinks(m.Text, m.HTML),
  };
}

/** Email yang dikirim ke `to` (terbaru dulu). */
export async function mailsTo(request: APIRequestContext, to: string): Promise<Mail[]> {
  const res = await request.get(`${MAILPIT_URL}/api/v1/search`, { params: { query: `to:"${to}"` } });
  expect(res.ok(), 'search mailpit').toBeTruthy();
  const summaries = ((await res.json()) as { messages: ApiSummary[] }).messages ?? [];
  // Pencarian Mailpit cocok sebagian; saring lagi supaya benar-benar penerima ini.
  const mine = summaries.filter((s) => s.To.some((a) => a.Address.toLowerCase() === to.toLowerCase()));
  return Promise.all(mine.map((s) => fetchMessage(request, s.ID)));
}

/**
 * Menunggu email ke `to` (opsional: subjek memuat `subject`) lalu mengembalikannya. Email dikirim asinkron oleh
 * notify-service, jadi di-poll sampai `timeoutMs`. Gagal dengan pesan jelas bila tidak muncul.
 */
export async function waitForMail(
  request: APIRequestContext,
  to: string,
  opts: { subject?: string | RegExp; timeoutMs?: number } = {},
): Promise<Mail> {
  const { subject, timeoutMs = 15_000 } = opts;
  const deadline = Date.now() + timeoutMs;
  let seen: string[] = [];
  for (;;) {
    const mails = await mailsTo(request, to);
    seen = mails.map((m) => m.subject);
    const hit = mails.find((m) =>
      subject === undefined ? true : typeof subject === 'string' ? m.subject.includes(subject) : subject.test(m.subject),
    );
    if (hit) return hit;
    if (Date.now() > deadline) {
      throw new Error(
        `Tidak ada email ke ${to}${subject ? ` dengan subjek ${subject}` : ''} dalam ${timeoutMs} ms. Subjek yang ada: ${JSON.stringify(seen)}`,
      );
    }
    await new Promise((r) => setTimeout(r, 300));
  }
}

/** Memastikan TIDAK ada email ke `to` dalam `windowMs` (mis. join ulang tidak boleh mengirim email ganda). */
export async function expectNoMail(request: APIRequestContext, to: string, windowMs = 2000): Promise<void> {
  await new Promise((r) => setTimeout(r, windowMs));
  expect(await mailsTo(request, to), `email tak terduga ke ${to}`).toHaveLength(0);
}

/** Mengirim email lewat API HTTP Mailpit (hanya untuk menguji helper ini; email aplikasi lewat SMTP). */
export async function sendViaMailpitApi(
  request: APIRequestContext,
  mail: { to: string; subject: string; text: string; html?: string },
): Promise<void> {
  const res = await request.post(`${MAILPIT_URL}/api/v1/send`, {
    data: {
      From: { Email: 'no-reply@qms.local', Name: 'Antrian' },
      To: [{ Email: mail.to }],
      Subject: mail.subject,
      Text: mail.text,
      HTML: mail.html,
    },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}
