/**
 * cn: penggabung class Tailwind (NativeWind) yang menyelesaikan konflik antar class.
 *
 * Dipakai oleh : src/components/*.tsx dan src/components/layout/ResponsiveScreenLayout.tsx.
 * Bergantung   : tailwind-merge (extendTailwindMerge). Radius kustom dari constants/tokens.js ikut didaftarkan.
 */

import { extendTailwindMerge } from 'tailwind-merge';

// Daftarkan radius kustom agar `rounded-pill` menimpa `rounded-button` (bukan keduanya ikut).
const twMerge = extendTailwindMerge({
  extend: { theme: { radius: ['input', 'button', 'card', 'pill'] } },
});

/** Nilai class yang boleh diberikan: string, atau false/null/undefined untuk class yang dilewati. */
type ClassValue = string | false | null | undefined;

/** Gabungkan class Tailwind; class belakangan menang atas yang bertabrakan (mis. override tinggi tombol). */
export function cn(...parts: ClassValue[]): string {
  return twMerge(parts.filter(Boolean).join(' '));
}
