/**
 * ThemedText: komponen teks bertema dari sisa template Expo.
 *
 * Dipakai oleh : tidak ada
 * Bergantung   : constants/theme (Fonts, ThemeColor), hooks/use-theme
 * Peran        : semua
 * Catatan      : TIDAK DIPAKAI: sisa template Expo. Gaya di sini tidak mengikuti sistem desain aplikasi (NativeWind).
 */
import { Platform, StyleSheet, Text, type TextProps } from 'react-native';

import { Fonts, ThemeColor } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/** Props teks bertema (sisa template Expo, tidak dipakai). */
export type ThemedTextProps = TextProps & {
  /** Varian gaya teks: ukuran dan ketebalan dari tabel `styles`; default `'default'`. */
  type?: 'default' | 'title' | 'small' | 'smallBold' | 'subtitle' | 'link' | 'linkPrimary' | 'code';
  /** Kunci warna dari tema (mis. `'text'`); default `'text'` bila tidak diisi. */
  themeColor?: ThemeColor;
};

/** Teks yang warnanya diambil dari tema aktif; gaya `style` dari luar selalu diterapkan paling akhir. */
export function ThemedText({ style, type = 'default', themeColor, ...rest }: ThemedTextProps) {
  const theme = useTheme();

  return (
    <Text
      style={[
        { color: theme[themeColor ?? 'text'] },
        type === 'default' && styles.default,
        type === 'title' && styles.title,
        type === 'small' && styles.small,
        type === 'smallBold' && styles.smallBold,
        type === 'subtitle' && styles.subtitle,
        type === 'link' && styles.link,
        type === 'linkPrimary' && styles.linkPrimary,
        type === 'code' && styles.code,
        style,
      ]}
      {...rest}
    />
  );
}

const styles = StyleSheet.create({
  small: {
    fontSize: 14,
    lineHeight: 20,
    fontWeight: 500,
  },
  smallBold: {
    fontSize: 14,
    lineHeight: 20,
    fontWeight: 700,
  },
  default: {
    fontSize: 16,
    lineHeight: 24,
    fontWeight: 500,
  },
  title: {
    fontSize: 48,
    fontWeight: 600,
    lineHeight: 52,
  },
  subtitle: {
    fontSize: 32,
    lineHeight: 44,
    fontWeight: 600,
  },
  link: {
    lineHeight: 30,
    fontSize: 14,
  },
  linkPrimary: {
    lineHeight: 30,
    fontSize: 14,
    color: '#3c87f7',
  },
  code: {
    fontFamily: Fonts.mono,
    // Android memakai bold 700 untuk font mono; platform lain cukup 500.
    fontWeight: Platform.select({ android: 700 }) ?? 500,
    fontSize: 12,
  },
});
