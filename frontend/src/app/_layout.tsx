import { DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import Head from 'expo-router/head';
import { StatusBar } from 'expo-status-bar';
import { useColorScheme } from 'nativewind';
import { useEffect } from 'react';

import { AppColors } from '@/constants/app-theme';
import { themedColors } from '@/constants/tokens';
import { restoreThemeMode } from '@/hooks/use-theme-mode';
import "../global.css";

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
      <Stack screenOptions={{ headerShown: false }} />
    </ThemeProvider>
  );
}
