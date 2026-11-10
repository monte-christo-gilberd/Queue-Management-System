/**
 * LogoMark: logo aplikasi berbentuk dua lingkaran antrean, digambar dengan View.
 *
 * Dipakai oleh : components/auth/AuthLayout.tsx, components/ui/PageWatermark.tsx
 * Bergantung   : constants/app-theme
 */
import { View } from 'react-native';

import { AppColors } from '@/constants/app-theme';

type LogoMarkProps = {
  /** Tinggi logo dalam piksel; lebarnya 1.4 kali nilai ini. Default 72. */
  size?: number;
  /** Warna garis/isi logo (default putih untuk latar gelap). */
  color?: string;
};

/** Logo mark aplikasi: 2 lingkaran antrean (belakang pudar, depan bercincin). Tanpa plat latar. */
export function LogoMark({ size = 72, color = AppColors.white }: LogoMarkProps) {
  // Ketebalan cincin minimal 2 piksel agar tetap terlihat di ukuran kecil.
  const ring = Math.max(2, size * 0.045);
  // Lingkaran belakang berukuran 68% dari lingkaran depan.
  const back = size * 0.68;
  // Semua ukuran dan warna diturunkan dari prop → lewat style; posisi/bentuk lewat className.
  return (
    <View style={{ width: size * 1.4, height: size }}>
      <View
        className="absolute right-0 rounded-full opacity-50"
        style={{ top: size * 0.16, width: back, height: back, borderWidth: ring, borderColor: color }}
      />
      <View
        className="absolute left-0 top-0 items-center justify-center rounded-full"
        style={{ width: size, height: size, borderWidth: ring, borderColor: color }}>
        <View
          className="rounded-full"
          style={{ width: size * 0.4, height: size * 0.4, backgroundColor: color }}
        />
      </View>
    </View>
  );
}
