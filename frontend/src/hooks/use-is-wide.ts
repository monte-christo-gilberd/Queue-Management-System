/**
 * use-is-wide: penanda layar lebar untuk memilih tata letak desktop atau ponsel. Menanggapi perubahan ukuran jendela.
 *
 * Dipakai oleh : src/app/(owner)/dashboard.tsx, src/components/HistoryDetail.tsx, src/components/History.tsx,
 *                src/components/layout/ResponsiveScreenLayout.tsx, src/components/Profile.tsx, src/components/QueueLiveScreen.tsx,
 *                src/components/ui/Toast.tsx.
 * Bergantung   : react-native (useWindowDimensions), constants/app-theme (Layout.wideBreakpoint).
 * Catatan      : Ambang lebar berasal dari dua sumber: `Layout.wideBreakpoint` (1024) untuk lebar umum, dan TRIPLE_BREAKPOINT (1280) untuk tiga kolom.
 */

import { useWindowDimensions } from 'react-native';

import { Layout } from '@/constants/app-theme';

/** true kalau lebar layar cukup untuk tampilan desktop (>= 1024px). */
export function useIsWide(): boolean {
  const { width } = useWindowDimensions();
  return width >= Layout.wideBreakpoint;
}

/** Lebar minimal untuk tata letak tiga kolom di detail antrian (semua kartu muat tanpa scroll di zoom 100%). */
export const TRIPLE_BREAKPOINT = 1280;

/** true bila layar cukup lebar untuk tiga kolom (>= 1280px). */
export function useIsTriple(): boolean {
  const { width } = useWindowDimensions();
  return width >= TRIPLE_BREAKPOINT;
}
