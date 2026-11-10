/**
 * use-hydrated: penanda bahwa klien sudah siap (sudah hydrate), untuk menunda layout yang bergantung pada peramban.
 *
 * Dipakai oleh : src/components/layout/ResponsiveScreenLayout.tsx.
 * Bergantung   : react-native (Platform).
 * Alur         : nilai awal `false` di web, `true` di native. Effect mengubahnya ke `true` setelah render pertama.
 * Catatan      : Pada render statis dan hydrate pertama, web mengembalikan false supaya markup server dan klien sama.
 */

import { useEffect, useState } from 'react';
import { Platform } from 'react-native';

/**
 * false selama render statis di server dan render pertama di browser (web), true setelah klien siap.
 * Di native selalu true. Dipakai agar layout tidak "berkedip" memakai lebar layar bawaan HP.
 */
export function useHydrated(): boolean {
  const [hydrated, setHydrated] = useState(Platform.OS !== 'web');
  useEffect(() => {
    setHydrated(true);
  }, []);
  return hydrated;
}
