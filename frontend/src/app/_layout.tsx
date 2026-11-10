/**
 * _layout: layout akar aplikasi: membungkus semua rute dengan tema, judul halaman, status bar, toast, dan Stack navigator.
 *
 * Rute        : semua rute (tidak punya URL sendiri).
 * Akses       : publik; tidak ada AuthGate di sini, penjagaan login ada di (owner)/_layout.tsx dan AuthGate per halaman.
 * Dipakai oleh: Expo Router sebagai pintu masuk setiap rute.
 * Bergantung  : ThemeProvider, Stack, ToastProvider dan ToastViewport, restoreThemeMode (hooks/use-theme-mode), global.css (NativeWind).
 * Catatan     : Pintu masuk arsitektur; urutan pembungkusan dijelaskan di JSDoc RootLayout.
 */
import { DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import Head from 'expo-router/head';
import { StatusBar } from 'expo-status-bar';
import { useColorScheme } from 'nativewind';
import { useEffect } from 'react';

import { ToastProvider, ToastViewport } from '@/components/ui/Toast';
import { AppColors } from '@/constants/app-theme';
import { themedColors } from '@/constants/tokens';
import { restoreThemeMode } from '@/hooks/use-theme-mode';
import "../global.css";

/**
 * Layout akar. Urutan pembungkusan dari luar ke dalam:
 * ThemeProvider (warna latar navigasi mengikuti mode terang atau gelap), Head (judul tab web), StatusBar,
 * lalu ToastProvider yang membungkus Stack (tanpa header bawaan) dan ToastViewport untuk galat berbentuk toast.
 * restoreThemeMode dipanggil sekali saat mount untuk memulihkan pilihan tema yang tersimpan.
 */
export default function RootLayout() {
  const { colorScheme } = useColorScheme();
  // Latar navigasi mengikuti tema agar tidak ada kilatan warna saat transisi/overscroll.
  const AppTheme = {
    ...DefaultTheme,
    colors: {
      ...DefaultTheme.colors,
      background: colorScheme === 'dark' ? themedColors.background.dark : AppColors.background,
    },
  };

  useEffect(() => {
    void restoreThemeMode();
  }, []);

  return (
    <ThemeProvider value={AppTheme}>
      {/* Judul tab browser (web); tanpa ini navigasi klien mengosongkan judul. */}
      <Head>
        <title>Queue Management System</title>
      </Head>
      <StatusBar style="light" />
      <ToastProvider>
        <Stack screenOptions={{ headerShown: false }} />
        {/* Toast galat: tumpukan di bawah layar, hilang sendiri setelah 5 detik. */}
        <ToastViewport />
      </ToastProvider>
    </ThemeProvider>
  );
}
