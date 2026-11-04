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
