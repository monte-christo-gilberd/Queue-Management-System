/**
 * use-theme: palet warna sesuai skema sistem (terang atau gelap), untuk komponen yang memakai style.
 *
 * Dipakai oleh : src/components/themed-text.tsx, src/components/themed-view.tsx.
 * Bergantung   : constants/theme (Colors), hooks/use-color-scheme (skema sistem).
 * Catatan      : Skema 'unspecified' dianggap 'light'. Mode yang dipilih pengguna ada di use-theme-mode.ts, bukan di sini.
 */

import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';

/**
 * Palet warna untuk skema aktif. Tanpa argumen. Mengembalikan objek `Colors` untuk 'light' atau 'dark'.
 */
export function useTheme() {
  const scheme = useColorScheme();
  const theme = scheme === 'unspecified' ? 'light' : scheme;

  return Colors[theme];
}
