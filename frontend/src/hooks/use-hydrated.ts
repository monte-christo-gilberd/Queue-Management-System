/**
 * use-hydrated: penanda bahwa klien sudah siap (sudah hydrate), untuk menunda layout yang bergantung pada peramban.
 *
 * Dipakai oleh : src/components/layout/ResponsiveScreenLayout.tsx.
 * Bergantung   : react (useSyncExternalStore).
 * Alur         : snapshot klien selalu `true`; snapshot server `false`. Di web, render statis dan hydrate pertama
 *                memakai `false` supaya markup server dan klien sama, lalu React beralih ke `true`. Di native tidak
 *                ada render server, jadi nilainya `true` sejak awal.
 * Catatan      : Tanpa effect, sehingga tidak ada setState saat render/effect dan tidak memicu render berantai.
 */

import { useSyncExternalStore } from 'react';

/** Tidak ada sumber eksternal: satu-satunya perubahan nilai terjadi saat React beralih dari snapshot server ke klien. */
function subscribe(): () => void {
  return () => {};
}

/** Di klien (setelah hydrate) selalu siap. */
function getSnapshot(): boolean {
  return true;
}

/** Saat render statis di server, klien belum siap. */
function getServerSnapshot(): boolean {
  return false;
}

/**
 * false selama render statis di server dan hydrate pertama di browser (web), true setelah klien siap.
 * Di native selalu true. Dipakai agar layout tidak "berkedip" memakai lebar layar bawaan HP.
 */
export function useHydrated(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
