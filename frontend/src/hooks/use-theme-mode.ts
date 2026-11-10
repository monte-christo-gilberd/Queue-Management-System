/**
 * use-theme-mode: pilihan mode tema terang atau gelap milik pengguna, yang disimpan di perangkat.
 *
 * Dipakai oleh : src/app/_layout.tsx (restoreThemeMode), src/app/(owner)/antrian.tsx, src/app/(owner)/riwayat.tsx,
 *                src/components/History.tsx, src/components/layout/ResponsiveScreenLayout.tsx, src/components/QueueQRCard.tsx,
 *                src/components/ui/AppButton.tsx.
 * Bergantung   : @react-native-async-storage/async-storage, nativewind (colorScheme, useColorScheme), constants/tokens (themedColors), store/safe-storage.
 * Alur         : app dibuka -> restoreThemeMode membaca 'qms-theme' -> colorScheme NativeWind. Toggle -> NativeWind dan storage.
 * Catatan      : Penyimpanan memakai createSafeStorage, sehingga galat storage tidak menghentikan aplikasi.
 */

import AsyncStorage from '@react-native-async-storage/async-storage';
import { colorScheme, useColorScheme } from 'nativewind';
import { useCallback } from 'react';

import { themedColors } from '@/constants/tokens';
import { createSafeStorage } from '@/store/safe-storage';

/**
 * Mode tema (terang/gelap). NativeWind memegang mode aktif (kelas `.dark` di web, skema warna di native);
 * pilihan pengguna disimpan supaya bertahan saat aplikasi dibuka ulang.
 */
/** Kunci penyimpanan pilihan tema. Nilainya 'light' atau 'dark'. */
const THEME_KEY = 'qms-theme';
/** Penyimpanan yang tahan galat, dibungkus createSafeStorage. */
const storage = createSafeStorage(AsyncStorage);

/** Pilihan tema yang bisa disimpan pengguna. */
export type ThemeMode = 'light' | 'dark';

/**
 * Memulihkan mode yang disimpan. Dipanggil sekali di root layout. Tidak ada data → ikut default sistem.
 * Nilai selain 'light' atau 'dark' diabaikan, sehingga data rusak tidak mengubah tema.
 */
export async function restoreThemeMode(): Promise<void> {
  const saved = await storage.getItem(THEME_KEY);
  if (saved === 'light' || saved === 'dark') colorScheme.set(saved);
}

/**
 * Mode gelap aktif dan fungsi untuk membalik mode (sekaligus menyimpannya).
 *
 * Tanpa argumen. Mengembalikan objek dengan field:
 * - `dark`: true bila mode gelap aktif.
 * - `toggle()`: membalik mode dan menyimpan pilihan baru ke storage.
 */
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

/**
 * Warna tema untuk prop non-style (warna ikon, spinner) sesuai mode aktif. Nilai ada di tokens.js (themedColors).
 *
 * Argumen: `name` adalah nama token warna di themedColors. Mengembalikan kode warna untuk mode aktif.
 */
export function useThemeColor(name: keyof typeof themedColors): string {
  const { dark } = useThemeMode();
  return dark ? themedColors[name].dark : themedColors[name].light;
}
