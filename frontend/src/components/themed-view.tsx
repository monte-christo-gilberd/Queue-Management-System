/**
 * ThemedView: komponen View bertema dari sisa template Expo.
 *
 * Dipakai oleh : tidak ada
 * Bergantung   : constants/theme (ThemeColor), hooks/use-theme
 * Peran        : semua
 * Catatan      : TIDAK DIPAKAI: sisa template Expo. Tidak ada yang mengimpor berkas ini.
 */
import { View, type ViewProps } from 'react-native';

import { ThemeColor } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

export type ThemedViewProps = ViewProps & {
  /** Warna latar mode terang; tidak dipakai oleh implementasi saat ini. */
  lightColor?: string;
  /** Warna latar mode gelap; tidak dipakai oleh implementasi saat ini. */
  darkColor?: string;
  /** Kunci warna tema untuk latar; default `'background'` bila tidak diisi. */
  type?: ThemeColor;
};

/** View dengan warna latar dari tema aktif. */
export function ThemedView({ style, lightColor, darkColor, type, ...otherProps }: ThemedViewProps) {
  const theme = useTheme();

  return <View style={[{ backgroundColor: theme[type ?? 'background'] }, style]} {...otherProps} />;
}
