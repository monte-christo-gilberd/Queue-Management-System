import { apiFetch } from '@/lib/api';

/** Web Push hanya ada di browser (HTTPS atau localhost). Expo Go tidak mendukungnya. */
export function pushSupported(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    'serviceWorker' in navigator &&
    typeof window !== 'undefined' &&
    'PushManager' in window &&
    'Notification' in window
  );
}

/** Kunci VAPID base64url → Uint8Array untuk pushManager.subscribe. */
export function urlBase64ToBytes(base64: string): Uint8Array {
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  const raw = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

async function registration(): Promise<ServiceWorkerRegistration> {
  await navigator.serviceWorker.register('/sw.js');
  return navigator.serviceWorker.ready;
}

export async function isSubscribed(): Promise<boolean> {
  if (!pushSupported()) return false;
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  return !!(reg && (await reg.pushManager.getSubscription()));
}

/** Bila browser sudah berlangganan, daftarkan ulang ke akun yang sedang login (endpoint pindah ke akun ini). */
export async function syncSubscription(): Promise<boolean> {
  if (!pushSupported()) return false;
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return false;
  await apiFetch('/api/v1/push/subscribe', { method: 'POST', body: sub.toJSON() });
  return true;
}

/** Minta izin, berlangganan, lalu simpan ke backend. Mengembalikan false bila izin ditolak. */
export async function enablePush(): Promise<boolean> {
  if (!pushSupported()) return false;
  if ((await Notification.requestPermission()) !== 'granted') return false;
  const { public_key } = await apiFetch<{ public_key: string }>('/api/v1/push/public-key');
  const reg = await registration();
  const sub =
    (await reg.pushManager.getSubscription()) ??
    (await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToBytes(public_key) as BufferSource,
    }));
  await apiFetch('/api/v1/push/subscribe', { method: 'POST', body: sub.toJSON() });
  return true;
}

/** Dipanggil sebelum logout (masih butuh token): lepas langganan agar akun lama tidak menerima push. Gagal diabaikan. */
export async function releasePushForLogout(): Promise<void> {
  if (!pushSupported()) return;
  try {
    await disablePush();
  } catch {
    // logout tidak boleh gagal karena push
  }
}

export async function disablePush(): Promise<void> {
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return;
  await apiFetch('/api/v1/push/unsubscribe', { method: 'POST', body: { endpoint: sub.endpoint } });
  await sub.unsubscribe();
}
