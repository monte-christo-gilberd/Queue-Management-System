/**
 * push.ts: langganan Web Push untuk owner di browser. Mengurus izin, berlangganan, sinkron ke akun, dan lepas langganan.
 *
 * Dipakai oleh : components/PushToggle.tsx (nyala, mati, sinkron), lib/qr.ts (enablePush),
 *                utils/owner-menu.ts (releasePushForLogout)
 * Bergantung   : lib/api.ts (apiFetch), service worker /sw.js (dilayani nginx dari public/)
 * Alur         : enablePush dipanggil saat QR dibuat. releasePushForLogout dipanggil sebelum logout, selagi token
 *                masih ada. syncSubscription dipanggil PushToggle saat komponen dipasang, untuk memindahkan endpoint ke akun ini.
 * Catatan      : Web Push hanya di browser (HTTPS atau localhost). Expo Go tidak mendukungnya. isSubscribed,
 *                syncSubscription, dan enablePush mengembalikan false bila pushSupported() false. disablePush tidak
 *                mengecek dukungan, jadi pemanggilnya harus memastikan dulu, atau lewat releasePushForLogout.
 */
import { apiFetch } from '@/lib/api';

/**
 * pushSupported: true bila browser punya Service Worker, PushManager, dan Notification.
 * Hanya mengecek kemampuan browser, bukan izin pengguna.
 */
export function pushSupported(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    'serviceWorker' in navigator &&
    typeof window !== 'undefined' &&
    'PushManager' in window &&
    'Notification' in window
  );
}

/**
 * urlBase64ToBytes: kunci VAPID base64url menjadi Uint8Array untuk pushManager.subscribe.
 * Contoh: urlBase64ToBytes('AQ') menghasilkan Uint8Array [1].
 */
export function urlBase64ToBytes(base64: string): Uint8Array {
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  const raw = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

/** Mendaftarkan /sw.js lalu menunggu sampai service worker aktif. Pendaftaran ulang aman dan mengembalikan pendaftaran yang sama. */
async function registration(): Promise<ServiceWorkerRegistration> {
  await navigator.serviceWorker.register('/sw.js');
  return navigator.serviceWorker.ready;
}

/**
 * isSubscribed: true bila browser ini punya langganan push aktif. Tidak mengecek akun mana yang memilikinya;
 * untuk itu gunakan syncSubscription.
 */
export async function isSubscribed(): Promise<boolean> {
  if (!pushSupported()) return false;
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  return !!(reg && (await reg.pushManager.getSubscription()));
}

/**
 * syncSubscription: bila browser sudah berlangganan, daftarkan ulang ke akun yang sedang login (endpoint pindah ke akun ini).
 * Mengembalikan false bila belum ada langganan di browser ini.
 */
export async function syncSubscription(): Promise<boolean> {
  if (!pushSupported()) return false;
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return false;
  await apiFetch('/api/v1/push/subscribe', { method: 'POST', body: sub.toJSON() });
  return true;
}

/**
 * enablePush: minta izin, berlangganan, lalu simpan ke backend. Mengembalikan false bila izin ditolak.
 * Langganan yang sudah ada dipakai ulang, supaya tidak membuat endpoint baru.
 */
export async function enablePush(): Promise<boolean> {
  if (!pushSupported()) return false;
  // Izin ditolak: tidak ada langganan dan backend tidak diberi tahu.
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

/**
 * disablePush: membatalkan langganan di backend dulu, baru di browser. Tanpa langganan tidak melakukan apa-apa.
 * Backend didahulukan karena request-nya butuh token, dan langganan di browser baru dibatalkan setelah endpoint diterima backend.
 */
export async function disablePush(): Promise<void> {
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return;
  await apiFetch('/api/v1/push/unsubscribe', { method: 'POST', body: { endpoint: sub.endpoint } });
  await sub.unsubscribe();
}
