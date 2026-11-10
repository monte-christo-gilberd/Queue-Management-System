/**
 * app-theme: alias warna dan ukuran layout untuk prop non-style (ikon, placeholderTextColor, spinner).
 *
 * Dipakai oleh : src/components/*.tsx (AppColors), src/components/layout/ResponsiveScreenLayout.tsx,
 *                src/hooks/use-is-wide.ts (Layout), src/app/_layout.tsx.
 * Bergantung   : constants/tokens.js (sumber tunggal token warna dan breakpoint).
 * Catatan      : Untuk style gunakan className (NativeWind), bukan AppColors.
 */

import { colors, desktopMaxWidth, wideBreakpoint } from './tokens';

/**
 * Warna mockup (nilai dari tokens.js, sama dengan class Tailwind `bg-primary` dst).
 * Hanya dipakai untuk prop non-style (color ikon, placeholderTextColor, spinner); untuk style pakai className.
 */
export const AppColors = colors;

/** Layout layar lebar (PC/tablet). Dipakai hooks/use-is-wide.ts untuk menentukan tata letak. */
export const Layout = {
  /** Lebar minimum layar (piksel) dianggap layar lebar. */
  wideBreakpoint,
  /** Lebar maksimal konten di layar lebar (piksel). */
  desktopMaxWidth,
} as const;
