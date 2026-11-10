/**
 * use-auth-hydrated: penanda bahwa state auth sudah selesai dibaca dari storage, dipakai untuk menunda keputusan redirect.
 *
 * Dipakai oleh : src/components/auth/AuthGate.tsx.
 * Bergantung   : react (useSyncExternalStore), store/authStore (useAuthStore.persist).
 * Alur         : storage (AsyncStorage di native, localStorage di web) -> persist.onFinishHydration -> nilai true.
 * Catatan      : useSyncExternalStore dipakai supaya nilai langsung ikut berubah saat hydration selesai, tanpa effect.
 */

import { useSyncExternalStore } from 'react';

import { useAuthStore } from '@/store/authStore';

/**
 * `true` setelah `token` selesai dibaca dari storage (AsyncStorage; di web: localStorage).
 *
 * Sebelum itu `token` masih `null` walau sebenarnya user sudah login, jadi guard tidak boleh
 * memutuskan redirect. Saat render server (static export) dan render hydrate pertama di klien,
 * nilainya `false`, sehingga markup keduanya sama dan tidak terjadi mismatch.
 */
export function useAuthHydrated(): boolean {
  return useSyncExternalStore(
    // onFinishHydration mengembalikan fungsi unsubscribe, sesuai kontrak subscribe.
    (onChange) => useAuthStore.persist.onFinishHydration(onChange),
    () => useAuthStore.persist.hasHydrated(),
    () => false,
  );
}
