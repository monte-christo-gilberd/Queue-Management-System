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
