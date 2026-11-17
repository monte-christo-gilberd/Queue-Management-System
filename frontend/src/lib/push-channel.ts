/**
 * push-channel.ts: facade notifikasi giliran untuk WEB (Web Push). Versi Android ada di push-channel.native.ts;
 * Metro memilih file .native.ts di native, dan file ini di web.
 *
 * Dipakai oleh : components/PushToggle.tsx, utils/owner-menu.ts (releasePushForLogout).
 * Bergantung   : lib/push.ts (diteruskan apa adanya; perilaku Web Push tidak berubah).
 * Catatan      : kedua file harus mengekspor nama yang sama.
 */
export { disablePush, enablePush, pushSupported, releasePushForLogout } from './push';
export { syncSubscription as syncPush } from './push';

/** deniedMessage: pesan saat pengguna menolak izin notifikasi. */
export const deniedMessage = 'Izin notifikasi ditolak di browser';
