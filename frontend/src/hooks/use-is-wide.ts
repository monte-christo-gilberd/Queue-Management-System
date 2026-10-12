import { useWindowDimensions } from 'react-native';

import { Layout } from '@/constants/app-theme';

/** true kalau lebar layar cukup untuk tampilan desktop (>= 1024px). */
export function useIsWide(): boolean {
  const { width } = useWindowDimensions();
  return width >= Layout.wideBreakpoint;
}
