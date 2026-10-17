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

export type BasicStorage = {
  getItem: (name: string) => Promise<string | null> | string | null;
  setItem: (name: string, value: string) => Promise<void> | void;
  removeItem: (name: string) => Promise<void> | void;
};

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
