import { create } from 'zustand';

/**
 * Penanda foto profil milik user yang login (avatar_updated_at dari GET /users/me), dibagi ke semua layar
 * supaya header ikut berubah begitu foto diunggah atau dihapus di profil. Tidak dipersist: dimuat ulang dari
 * server setiap sesi, dan dikosongkan saat logout agar foto user sebelumnya tidak sempat tampil.
 */
type AvatarState = {
  updatedAt: string | null;
  setUpdatedAt: (value: string | null) => void;
};

export const useAvatarStore = create<AvatarState>()((set) => ({
  updatedAt: null,
  setUpdatedAt: (value) => set({ updatedAt: value }),
}));
