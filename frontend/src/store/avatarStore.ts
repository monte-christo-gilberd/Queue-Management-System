/**
 * avatarStore.ts: penanda cache foto profil user yang sedang login.
 *
 * Dipakai oleh : components/ui/UserAvatar.tsx (membaca), app/(owner)/_layout.tsx (OwnerProvider, mengisi),
 *                app/(owner)/profile/index.tsx (mengisi setelah unggah atau hapus foto).
 * Bergantung   : zustand.
 * Alur         : diisi dari avatar_updated_at (GET /api/v1/users/me); dikosongkan oleh authStore.logout().
 */
import { create } from 'zustand';

/**
 * Penanda foto profil milik user yang login (avatar_updated_at dari GET /users/me), dibagi ke semua layar
 * supaya header ikut berubah begitu foto diunggah atau dihapus di profil. Tidak dipersist: dimuat ulang dari
 * server setiap sesi, dan dikosongkan saat logout agar foto user sebelumnya tidak sempat tampil.
 */
type AvatarState = {
  /** Waktu unggah foto terakhir (string ISO dari avatar_updated_at); null bila tidak ada foto atau belum dimuat. Perubahan nilainya menandakan foto perlu dimuat ulang. */
  updatedAt: string | null;
  /** Mengganti penanda; null menghapus penanda (dipakai saat logout dan setelah foto dihapus). */
  setUpdatedAt: (value: string | null) => void;
};

/**
 * useAvatarStore: store zustand untuk penanda foto profil. Tidak dipersist, sehingga tiap sesi mulai dari null.
 */
export const useAvatarStore = create<AvatarState>()((set) => ({
  updatedAt: null,
  setUpdatedAt: (value) => set({ updatedAt: value }),
}));
