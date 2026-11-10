/**
 * safe-storage.ts: pembungkus storage zustand persist yang tidak pernah melempar.
 *
 * Dipakai oleh : store/authStore.ts (storage persist `qms-auth`), hooks/use-theme-mode.ts (tema).
 * Catatan      : alasan hidrasi tidak menggantung dijelaskan di HYDRATION_GUIDE.md.
 */

/**
 * Pembungkus storage untuk zustand `persist` yang tidak pernah melempar.
 *
 * Kenapa perlu: bila `getItem` gagal (mis. storage rusak atau dibatasi browser), zustand tidak
 * menyelesaikan hydrate, sehingga `hasHydrated()` tetap `false` dan guard menampilkan loading
 * selamanya. Di sini kegagalan dibaca sebagai "tidak ada data" → hydrate selesai dengan state awal.
 *
 * Kegagalan tulis diabaikan: sesi hanya tidak bertahan setelah refresh, user cukup login ulang.
 *
 * Tidak mengimpor AsyncStorage langsung; storage dasar diterima sebagai argumen supaya bisa diuji di Node.
 */

/** BasicStorage: antarmuka storage dasar (mis. AsyncStorage atau localStorage) yang dibungkus createSafeStorage. */
export type BasicStorage = {
  /** Membaca nilai untuk kunci `name`; null bila kunci belum ada. */
  getItem: (name: string) => Promise<string | null> | string | null;
  /** Menulis `value` (string JSON) ke kunci `name`. */
  setItem: (name: string, value: string) => Promise<void> | void;
  /** Menghapus kunci `name`. */
  removeItem: (name: string) => Promise<void> | void;
};

/**
 * createSafeStorage membungkus `base` sehingga setiap operasi berjalan dalam try/catch.
 * Contoh: `getItem` yang melempar menghasilkan null, sehingga hidrasi selesai dengan state awal.
 */
export function createSafeStorage(base: BasicStorage): BasicStorage {
  return {
    getItem: async (name) => {
      try {
        return (await base.getItem(name)) ?? null;
      } catch {
        return null;
      }
    },
    setItem: async (name, value) => {
      try {
        await base.setItem(name, value);
      } catch {
        // diabaikan, lihat komentar di atas
      }
    },
    removeItem: async (name) => {
      try {
        await base.removeItem(name);
      } catch {
        // diabaikan
      }
    },
  };
}
