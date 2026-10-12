import { View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { PersonIcon } from '@/components/ui/Icon';
import { cn } from '@/utils/cn';

type AvatarProps = {
  size?: number;
  /** true = latar gelap (aktif), false = latar pink pudar (tidak aktif). */
  active?: boolean;
};

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
