/**
 * use-color-scheme: skema warna sistem (terang atau gelap) dari react-native untuk platform native.
 *
 * Dipakai oleh : src/hooks/use-theme.ts (lewat impor). Tidak dipanggil langsung dari komponen.
 * Bergantung   : react-native (useColorScheme).
 * Catatan      : Di web, Metro memakai use-color-scheme.web.ts sebagai pengganti berkas ini.
 */

export { useColorScheme } from 'react-native';
