/**
 * Avatar: lingkaran avatar generik berisi ikon orang, dengan dua warna latar (aktif / tidak aktif).
 *
 * Dipakai oleh : belum ada pemakai (tidak ditemukan impor ke komponen ini)
 * Bergantung   : constants/app-theme, components/ui/Icon (PersonIcon), utils/cn
 */
import { View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { PersonIcon } from '@/components/ui/Icon';
import { cn } from '@/utils/cn';

type AvatarProps = {
  /** Diameter lingkaran dalam piksel. Ikon orang di dalamnya 45% dari nilai ini. Default 48. */
  size?: number;
  /** true = latar gelap (aktif), false = latar pink pudar (tidak aktif). Default false. */
  active?: boolean;
};

/** Lingkaran avatar dengan ikon orang di tengah. */
export function Avatar({ size = 48, active = false }: AvatarProps) {
  return (
    <View
      className={cn('items-center justify-center', active ? 'bg-dark' : 'bg-avatar')}
      // Ukuran dinamis (prop `size`) tidak bisa jadi class statis Tailwind.
      style={{ width: size, height: size, borderRadius: size / 2 }}>
      <PersonIcon size={size * 0.45} color={AppColors.white} />
    </View>
  );
}
