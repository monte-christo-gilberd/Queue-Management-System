/**
 * use-color-scheme (versi web): skema warna sistem untuk web, dengan nilai awal yang aman untuk render statis.
 *
 * Dipakai oleh : src/hooks/use-theme.ts (lewat impor; Metro memilih berkas ini di web).
 * Bergantung   : react (useEffect, useState), react-native (useColorScheme).
 * Alur         : render pertama dan render statis -> 'light'. Setelah klien siap, effect mengganti dengan skema sistem sebenarnya.
 * Catatan      : Nilai sistem hanya dibaca di klien agar markup server dan hydrate pertama tidak berbeda.
 */

import { useEffect, useState } from 'react';
import { useColorScheme as useRNColorScheme } from 'react-native';

/**
 * Skema warna untuk web. Nilai dihitung ulang di sisi klien agar render statis tetap konsisten.
 *
 * Tanpa argumen. Mengembalikan 'light' atau 'dark' setelah klien siap, dan 'light' sebelum itu.
 */
export function useColorScheme() {
  const [hasHydrated, setHasHydrated] = useState(false);

  // Setelah render pertama di klien, tandai sudah hydrate supaya skema sistem dipakai.
  useEffect(() => {
    setHasHydrated(true);
  }, []);

  const colorScheme = useRNColorScheme();

  // Sebelum hydrate, selalu 'light' agar markup server dan klien sama.
  if (hasHydrated) {
    return colorScheme;
  }

  return 'light';
}
