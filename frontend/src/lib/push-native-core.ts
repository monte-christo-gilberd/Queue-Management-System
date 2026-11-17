/**
 * push-native-core.ts: logika notifikasi giliran di aplikasi Android (FCM), tanpa impor Expo/React Native.
 *
 * Dipakai oleh : lib/push-native.ts (merangkai dengan expo-notifications, AsyncStorage, dan apiFetch).
 * Kenapa       : dependensi disuntikkan supaya alur izin, token, daftar, dan lepas bisa dites di Node
 *                (tests/unit/push-native.test.ts) tanpa modul native.
 * Alur         : enable -> buat kanal `queue-turn` -> minta izin POST_NOTIFICATIONS -> ambil token FCM ->
 *                POST /api/v1/push/fcm/register -> simpan token lokal. Token lokal = tanda notifikasi aktif dan
 *                dipakai saat unregister (tidak perlu meminta token lagi saat logout atau offline).
 */
import type { BasicStorage } from '@/store/safe-storage';

/** CHANNEL_ID: kanal notifikasi Android. Harus sama dengan channel_id di notify-service (internal/fcm). */
export const CHANNEL_ID = 'queue-turn';

/** STORAGE_KEY: kunci penyimpanan token FCM yang terakhir didaftarkan. Ada = notifikasi aktif. */
export const STORAGE_KEY = 'qms-push-native-token';

/** NativePushDeps: kemampuan platform yang dibutuhkan logika ini. */
export type NativePushDeps = {
  /** available: false di Expo Go, iOS, atau web (FCM tidak tersedia). */
  available: boolean;
  /** ensureChannel: membuat kanal `queue-turn` (importance HIGH). Aman dipanggil berulang. */
  ensureChannel: () => Promise<void>;
  /** permissionGranted: status izin notifikasi saat ini, tanpa meminta. */
  permissionGranted: () => Promise<boolean>;
  /** requestPermission: meminta izin (dialog Android 13+); true bila diberikan. */
  requestPermission: () => Promise<boolean>;
  /** deviceToken: token FCM mentah perangkat (getDevicePushTokenAsync). */
  deviceToken: () => Promise<string>;
  /** api: request ke backend dengan token login (apiFetch). */
  api: (path: string, init: { method: 'POST'; body: unknown }) => Promise<unknown>;
  /** storage: penyimpanan lokal (AsyncStorage). */
  storage: BasicStorage;
};

/** NativePush: operasi notifikasi native yang dipakai facade lib/push-channel.native.ts. */
export type NativePush = ReturnType<typeof createNativePush>;

/** createNativePush merangkai operasi notifikasi native dari dependensi `deps`. */
export function createNativePush(deps: NativePushDeps) {
  /** Simpan token di backend untuk akun yang login (token yang sama dipindah dari akun lain). */
  const register = (token: string) =>
    deps.api('/api/v1/push/fcm/register', { method: 'POST', body: { token, platform: 'android' } });
  /** Lepas token dari akun yang login; backend hanya menghapus token milik akun itu. */
  const unregister = (token: string) =>
    deps.api('/api/v1/push/fcm/unregister', { method: 'POST', body: { token } });

  /** Daftarkan token ke akun yang login, lalu simpan sebagai token aktif. */
  const registerAndStore = async (token: string) => {
    await register(token);
    await deps.storage.setItem(STORAGE_KEY, token);
  };

  return {
    /** supported: true bila FCM tersedia (development/production build Android). */
    supported: () => deps.available,

    /** ensureChannel: buat kanal notifikasi saat startup, sebelum notifikasi pertama datang. Galat diabaikan. */
    ensureChannel: async () => {
      if (!deps.available) return;
      try {
        await deps.ensureChannel();
      } catch {
        // kanal dibuat ulang pada enable berikutnya
      }
    },

    /**
     * enable: buat kanal, minta izin, ambil token, daftarkan ke backend. false bila izin ditolak
     * (tidak ada token yang didaftarkan).
     */
    enable: async (): Promise<boolean> => {
      if (!deps.available) return false;
      // Kanal dulu: notifikasi yang datang tepat setelah token terdaftar sudah punya kanal untuk ditampilkan.
      await deps.ensureChannel();
      // Izin sudah ada: jangan tampilkan dialog lagi. Belum ada: minta (Android 13+ menampilkan dialog sistem).
      const granted = (await deps.permissionGranted()) || (await deps.requestPermission());
      // Ditolak: tidak ada token yang dikirim ke backend; PushToggle menampilkan deniedMessage.
      if (!granted) return false;
      // Ambil token FCM perangkat, daftarkan, lalu tandai aktif secara lokal.
      await registerAndStore(await deps.deviceToken());
      return true;
    },

    /** disable: lepas token di backend dulu (butuh login), baru hapus tanda lokal. Tanpa token tidak melakukan apa-apa. */
    disable: async (): Promise<void> => {
      const token = await deps.storage.getItem(STORAGE_KEY);
      // Belum pernah aktif di perangkat ini: tidak ada yang dilepas.
      if (!token) return;
      // Bila request gagal, galat dilempar dan tanda lokal tetap ada, jadi tombol tetap "aktif" (jujur ke pengguna).
      await unregister(token);
      await deps.storage.removeItem(STORAGE_KEY);
    },

    /**
     * sync: bila notifikasi aktif di perangkat ini dan izin masih ada, daftarkan ulang token ke akun yang login
     * (token pindah ke akun ini). false bila belum aktif atau izin dicabut dari Pengaturan.
     */
    sync: async (): Promise<boolean> => {
      if (!deps.available) return false;
      const stored = await deps.storage.getItem(STORAGE_KEY);
      // Pengguna belum pernah mengaktifkan notifikasi di perangkat ini.
      if (!stored) return false;
      // Izin dicabut dari Pengaturan: anggap mati dan lepas token supaya backend berhenti mengirim.
      if (!(await deps.permissionGranted())) {
        try {
          await unregister(stored);
        } catch {
          // dicoba lagi pada sync berikutnya
          return false;
        }
        await deps.storage.removeItem(STORAGE_KEY);
        return false;
      }
      const token = await deps.deviceToken();
      // Token berganti (mis. data aplikasi dihapus): lepas yang lama, baru daftarkan yang baru.
      if (token !== stored) await unregister(stored).catch(() => undefined);
      await registerAndStore(token);
      return true;
    },

    /** tokenChanged: dipanggil listener token FCM. Daftarkan token baru hanya bila notifikasi sedang aktif. */
    tokenChanged: async (token: string): Promise<void> => {
      if (!deps.available || !token) return;
      const stored = await deps.storage.getItem(STORAGE_KEY);
      // Notifikasi mati, atau token tidak berubah: tidak ada yang perlu didaftarkan.
      if (!stored || stored === token) return;
      try {
        await unregister(stored).catch(() => undefined);
        await registerAndStore(token);
      } catch {
        // gagal (mis. offline): sync saat layar tiket dibuka akan mencoba lagi
      }
    },

    /** releaseForLogout: lepas token sebelum logout selagi token login masih ada. Tidak pernah melempar. */
    releaseForLogout: async (): Promise<void> => {
      try {
        const token = await deps.storage.getItem(STORAGE_KEY);
        if (!token) return;
        await unregister(token);
        await deps.storage.removeItem(STORAGE_KEY);
      } catch {
        // logout tidak boleh gagal karena push
      }
    },
  };
}

/**
 * notificationUrl: path aplikasi dari data notifikasi (`data.url`, mis. `/ticket/12`). Hanya path internal
 * (diawali satu `/`) yang diterima, supaya notifikasi tidak bisa membuka URL luar. null bila tidak ada.
 */
export function notificationUrl(...candidates: unknown[]): string | null {
  for (const c of candidates) {
    if (typeof c === 'string' && c.startsWith('/') && !c.startsWith('//')) return c;
  }
  return null;
}
