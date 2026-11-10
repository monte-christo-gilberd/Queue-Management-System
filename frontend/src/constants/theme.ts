/**
 * theme: sisa template Expo berisi palet warna, skala spacing, dan font. Hanya `Fonts` yang masih dipakai.
 *
 * Dipakai oleh : src/components/QueueLiveScreen.tsx, src/components/MySpotCard.tsx,
 *                src/components/ui/MyTicketModal.tsx (Fonts).
 * Catatan      : Mengimpor '@/global.css' sebagai efek samping saat modul dimuat. Palet warna yang aktif ada di
 *                constants/tokens.js; bagian yang berstatus TIDAK DIPAKAI sengaja tidak dihapus.
 */

import '@/global.css';

import { Platform } from 'react-native';

/** TIDAK DIPAKAI: sisa template Expo. Satu-satunya pemakai adalah hooks/use-theme.ts, yang hanya dipakai berkas dead. */
export const Colors = {
  light: {
    text: '#000000',
    background: '#ffffff',
    backgroundElement: '#F0F0F3',
    backgroundSelected: '#E0E1E6',
    textSecondary: '#60646C',
  },
  dark: {
    text: '#ffffff',
    background: '#000000',
    backgroundElement: '#212225',
    backgroundSelected: '#2E3135',
    textSecondary: '#B0B4BA',
  },
} as const;

/** TIDAK DIPAKAI: tipe kunci warna tema, hanya dipakai components/themed-text.tsx dan themed-view.tsx (dead). */
export type ThemeColor = keyof typeof Colors.light & keyof typeof Colors.dark;

/** Keluarga font per platform. Nilai web memakai variabel CSS dari global.css. */
export const Fonts = Platform.select({
  ios: {
    /** iOS `UIFontDescriptorSystemDesignDefault` */
    sans: 'system-ui',
    /** iOS `UIFontDescriptorSystemDesignSerif` */
    serif: 'ui-serif',
    /** iOS `UIFontDescriptorSystemDesignRounded` */
    rounded: 'ui-rounded',
    /** iOS `UIFontDescriptorSystemDesignMonospaced` */
    mono: 'ui-monospace',
  },
  default: {
    sans: 'normal',
    serif: 'serif',
    rounded: 'normal',
    mono: 'monospace',
  },
  web: {
    sans: 'var(--font-display)',
    serif: 'var(--font-serif)',
    rounded: 'var(--font-rounded)',
    mono: 'var(--font-mono)',
  },
});

/** TIDAK DIPAKAI: skala spacing template Expo (nilai dalam piksel), tidak ada yang mengimpornya. */
export const Spacing = {
  half: 2,
  one: 4,
  two: 8,
  three: 16,
  four: 24,
  five: 32,
  six: 64,
} as const;

/** TIDAK DIPAKAI: tinggi inset tab bawah per platform, tidak ada yang mengimpornya. */
export const BottomTabInset = Platform.select({ ios: 50, android: 80 }) ?? 0;
/** TIDAK DIPAKAI: lebar maksimal konten dalam piksel, tidak ada yang mengimpornya. Lebar layar lebar ada di constants/app-theme.ts. */
export const MaxContentWidth = 800;
