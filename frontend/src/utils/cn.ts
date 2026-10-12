import { extendTailwindMerge } from 'tailwind-merge';

// Daftarkan radius kustom agar `rounded-pill` menimpa `rounded-button` (bukan keduanya ikut).
const twMerge = extendTailwindMerge({
  extend: { theme: { radius: ['input', 'button', 'card', 'pill'] } },
});

type ClassValue = string | false | null | undefined;

/** Gabungkan class Tailwind; class belakangan menang atas yang bertabrakan (mis. override tinggi tombol). */
export function cn(...parts: ClassValue[]): string {
  return twMerge(parts.filter(Boolean).join(' '));
}
