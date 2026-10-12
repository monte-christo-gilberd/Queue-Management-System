import { colors, desktopMaxWidth, wideBreakpoint } from './tokens';

/**
 * Warna mockup (nilai dari tokens.js — sama dengan class Tailwind `bg-primary` dst).
 * Hanya dipakai untuk prop non-style (color ikon, placeholderTextColor, spinner); untuk style pakai className.
 */
export const AppColors = colors;

/** Layout layar lebar (PC/tablet). */
export const Layout = {
  wideBreakpoint,
  desktopMaxWidth,
} as const;
