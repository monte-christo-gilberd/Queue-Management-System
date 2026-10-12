import { DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import Head from 'expo-router/head';
import { StatusBar } from 'expo-status-bar';

import { AppColors } from '@/constants/app-theme';
import "../global.css";

// Mockup hanya tersedia dalam tema terang, jadi tema gelap tidak dipakai.
// Latar navigasi mengikuti abu terang agar tidak ada kilatan putih saat transisi/overscroll.
const AppTheme = {
  ...DefaultTheme,
  colors: { ...DefaultTheme.colors, background: AppColors.background },
};

export default function RootLayout() {
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
