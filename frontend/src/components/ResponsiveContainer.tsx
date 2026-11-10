/**
 * ResponsiveContainer: pembatas lebar konten untuk layar tablet dan web.
 *
 * Dipakai oleh : components/layout/ResponsiveScreenLayout.tsx
 * Bergantung   : react-native (useWindowDimensions), utils/cn
 * Peran        : semua
 * Catatan      : lebar dibatasi lewat style karena nilai maxWidth dinamis, bukan class statis.
 */
import { useWindowDimensions, View } from 'react-native';

import { cn } from '@/utils/cn';

type ResponsiveContainerProps = {
  /** Konten yang dibungkus; dirender apa adanya di dalam View yang dibatasi lebarnya. */
  children: React.ReactNode;
  /** Lebar maksimum konten: 480 untuk layar form, 720+ untuk layar list/detail. */
  maxWidth?: number;
};

/**
 * Membatasi lebar konten dan menengahkannya di layar lebar (tablet/web).
 * Di layar selebar HP atau lebih sempit, tampilan dibiarkan full-width seperti aslinya.
 */
export default function ResponsiveContainer({ children, maxWidth = 480 }: ResponsiveContainerProps) {
  const { width } = useWindowDimensions();
  const constrained = width > maxWidth;

  // flex-1 menjaga tinggi layar penuh agar header/tab bar tetap di posisinya.
  // Lebar `maxWidth` dinamis (prop) sehingga lewat style, bukan class statis.
  return (
    <View
      className={cn('flex-1', constrained && 'self-center')}
      style={constrained ? { width: maxWidth } : undefined}>
      {children}
    </View>
  );
}
