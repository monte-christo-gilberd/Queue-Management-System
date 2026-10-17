import AsyncStorage from '@react-native-async-storage/async-storage';
import { colorScheme, useColorScheme } from 'nativewind';
import { useCallback } from 'react';

import { themedColors } from '@/constants/tokens';
import { createSafeStorage } from '@/store/safe-storage';

/**
 * Mode tema (terang/gelap). NativeWind memegang mode aktif (kelas `.dark` di web, skema warna di native);
 * pilihan pengguna disimpan supaya bertahan saat aplikasi dibuka ulang.
 */
const THEME_KEY = 'qms-theme';
const storage = createSafeStorage(AsyncStorage);

export type ThemeMode = 'light' | 'dark';

/** Memulihkan mode yang disimpan. Dipanggil sekali di root layout. Tidak ada data → ikut default sistem. */
export async function restoreThemeMode(): Promise<void> {
  const saved = await storage.getItem(THEME_KEY);
  if (saved === 'light' || saved === 'dark') colorScheme.set(saved);
}

/** Mode gelap aktif dan fungsi untuk membalik mode (sekaligus menyimpannya). */
export function useThemeMode() {
  const { colorScheme: scheme, setColorScheme } = useColorScheme();
  const dark = scheme === 'dark';

  const toggle = useCallback(() => {
    const next: ThemeMode = dark ? 'light' : 'dark';
    setColorScheme(next);
    void storage.setItem(THEME_KEY, next);
  }, [dark, setColorScheme]);

  return { dark, toggle };
}

/** Warna tema untuk prop non-style (warna ikon, spinner) sesuai mode aktif. Nilai ada di tokens.js (themedColors). */
export function useThemeColor(name: keyof typeof themedColors): string {
  const { dark } = useThemeMode();
  return dark ? themedColors[name].dark : themedColors[name].light;
}
