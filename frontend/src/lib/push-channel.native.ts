/**
 * push-channel.native.ts: facade notifikasi giliran untuk ANDROID (FCM). Versi web ada di push-channel.ts.
 *
 * Dipakai oleh : components/PushToggle.tsx, utils/menu.ts (releasePushForLogout).
 * Bergantung   : lib/push-native.ts.
 * Catatan      : kedua file harus mengekspor nama yang sama.
 */
import { nativePush } from './push-native';

/** pushSupported: true di development/production build Android; false di Expo Go dan iOS. */
export const pushSupported = (): boolean => nativePush.supported();
/** syncPush: daftarkan ulang token ke akun yang login bila notifikasi aktif di perangkat ini. */
export const syncPush = (): Promise<boolean> => nativePush.sync();
/** enablePush: minta izin lalu daftarkan token FCM. false bila izin ditolak. */
export const enablePush = (): Promise<boolean> => nativePush.enable();
/** disablePush: lepas token FCM dari akun ini. */
export const disablePush = (): Promise<void> => nativePush.disable();
/** releasePushForLogout: lepas token sebelum logout. Tidak pernah melempar. */
export const releasePushForLogout = (): Promise<void> => nativePush.releaseForLogout();

/** deniedMessage: pesan saat pengguna menolak izin notifikasi. */
export const deniedMessage =
  'Izin notifikasi ditolak. Aktifkan di Pengaturan > Aplikasi > Queue Management System > Notifikasi.';
