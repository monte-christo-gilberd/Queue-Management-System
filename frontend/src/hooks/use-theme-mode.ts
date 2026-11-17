/**
 * use-theme-mode: pilihan mode tema terang atau gelap milik pengguna, yang disimpan di perangkat.
 *
 * Dipakai oleh : src/app/_layout.tsx (restoreThemeMode), src/app/(user)/antrian.tsx, src/app/(user)/riwayat.tsx,
 *                src/components/History.tsx, src/components/layout/ResponsiveScreenLayout.tsx, src/components/QueueQRCard.tsx,
 *                src/components/ui/AppButton.tsx.
 * Bergantung   : @react-native-async-storage/async-storage, nativewind (colorScheme, useColorScheme), constants/tokens (themedColors), store/safe-storage.
 * Alur         : app dibuka -> restoreThemeMode membaca 'qms-theme' -> colorScheme NativeWind. Toggle -> NativeWind dan storage.
 * Catatan      : Penyimpanan memakai createSafeStorage, sehingga galat storage tidak menghentikan aplikasi.
 */

import AsyncStorage from '@react-native-async-storage/async-storage';
import { colorScheme, useColorScheme } from 'nativewind';
import { useCallback } from 'react';
import { Appearance, DeviceEventEmitter, Platform } from 'react-native';

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
 * Mengisi observable skema warna NativeWind langsung (native saja), tanpa menunggu event Appearance.
 *
 * Pendengar Appearance milik NativeWind mengabaikan perubahan bila AppState belum 'active' (mis. saat tema
 * tersimpan dipulihkan di awal aplikasi), dan di Android event native bisa terlambat atau tidak datang.
 * Dengan mengisi observable ini seluruh warna `dark:` dan variabel CSS bertema berganti saat itu juga.
 */
function setNativeWindScheme(mode: ThemeMode): void {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { systemColorScheme } = require('react-native-css-interop/dist/runtime/native/appearance-observables') as {
      systemColorScheme: { set: (value: ThemeMode) => void };
    };
    systemColorScheme.set(mode);
  } catch {
    // Struktur internal library berubah: jalur event di applyThemeMode tetap berlaku.
  }
}

/**
 * Menerapkan mode ke NativeWind.
 *
 * Di web, `colorScheme.set` cukup (mengganti kelas `.dark` pada <html>).
 * Di native (APK), `colorScheme.set` hanya memanggil `Appearance.setColorScheme`, dan NativeWind baru
 * memperbarui warna setelah event `appearanceChanged` dari sisi native. Event itu tidak selalu datang
 * di Android (bergantung pada perubahan konfigurasi activity), sehingga tema gelap tidak pernah aktif
 * di build release. Karena itu event yang sama dikirim langsung dari JS; RN dan NativeWind
 * mendengarkan kanal `RCTDeviceEventEmitter` yang sama, sehingga cara ini aman di semua platform native.
 * Pendengar itu juga mengabaikan perubahan bila AppState belum 'active', jadi `setNativeWindScheme`
 * mengisi skema warna NativeWind langsung sebagai jalur kedua.
 */
function applyThemeMode(mode: ThemeMode): void {
  colorScheme.set(mode);
  if (Platform.OS !== 'web') {
    (DeviceEventEmitter as unknown as { emit: (name: string, payload: unknown) => void }).emit(
      'appearanceChanged',
      { colorScheme: mode },
    );
    setNativeWindScheme(mode);
  }
}

/**
 * Memulihkan mode yang disimpan. Dipanggil sekali di root layout. Tidak ada data → ikut default sistem.
 * Nilai selain 'light' atau 'dark' diabaikan, sehingga data rusak tidak mengubah tema.
 */
export async function restoreThemeMode(): Promise<void> {
  const saved = await storage.getItem(THEME_KEY);
  if (saved === 'light' || saved === 'dark') {
    applyThemeMode(saved);
    return;
  }
  // Belum ada pilihan tersimpan: ikuti tema sistem perangkat (native). Dibaca eksplisit karena nilai awal
  // NativeWind bisa terbaca sebelum Android melaporkan mode gelap sistem.
  if (Platform.OS !== 'web') {
    const system = Appearance.getColorScheme();
    if (system === 'dark' || system === 'light') setNativeWindScheme(system);
  }
}

/**
 * Mode gelap aktif dan fungsi untuk membalik mode (sekaligus menyimpannya).
 *
 * Tanpa argumen. Mengembalikan objek dengan field:
 * - `dark`: true bila mode gelap aktif.
 * - `toggle()`: membalik mode dan menyimpan pilihan baru ke storage.
 */
export function useThemeMode() {
  const { colorScheme: scheme } = useColorScheme();
  const dark = scheme === 'dark';

  const toggle = useCallback(() => {
    const next: ThemeMode = dark ? 'light' : 'dark';
    applyThemeMode(next);
    void storage.setItem(THEME_KEY, next);
  }, [dark]);

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
