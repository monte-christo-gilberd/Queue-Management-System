/**
 * PageWatermark: latar dekoratif samar untuk kerangka layar owner.
 *
 * Dipakai oleh : components/layout/ResponsiveScreenLayout.tsx
 * Bergantung   : auth/LogoMark, constants/app-theme
 * Catatan      : Seluruh lapisan memakai pointerEvents none, jadi tidak menangkap sentuhan.
 */
import { View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { LogoMark } from '@/components/auth/LogoMark';

/**
 * Watermark latar halaman (di luar halaman auth): blob organik + logo mark besar,
 * sama dengan motif halaman login tetapi sangat samar di atas latar abu terang.
 */
export function PageWatermark() {
  return (
    <View className="absolute bottom-0 left-0 right-0 top-0 overflow-hidden" pointerEvents="none">
      <View
        className="absolute -left-[160px] -top-[180px] h-[460px] w-[560px] rounded-bl-[300px] rounded-br-[200px] rounded-tl-[260px] rounded-tr-[320px] bg-primary opacity-[0.07]"
        style={{ transform: [{ rotate: '-14deg' }] }}
      />
      <View
        className="absolute -bottom-[220px] -right-[180px] h-[520px] w-[640px] rounded-bl-[260px] rounded-br-[300px] rounded-tl-[340px] rounded-tr-[220px] bg-primary opacity-[0.06]"
        style={{ transform: [{ rotate: '18deg' }] }}
      />
      {/* Logo mark besar di tengah, hampir transparan. */}
      <View className="absolute bottom-0 left-0 right-0 top-0 items-center justify-center opacity-[0.08]">
        <LogoMark size={240} color={AppColors.primary} />
      </View>
    </View>
  );
}
