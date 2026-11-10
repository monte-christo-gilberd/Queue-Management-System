/**
 * UserAvatar: foto profil user yang sedang login, atau ikon orang bila belum ada foto.
 *
 * Dipakai oleh : components/layout/ResponsiveScreenLayout.tsx
 * Bergantung   : hooks/use-avatar-source, store/avatarStore (updatedAt), components/ui/Icon (PersonIcon)
 * Alur         : avatarStore.updatedAt -> useAvatarSource -> foto, atau ikon saat belum ada sumber
 * Catatan      : Membaca avatarStore langsung, di luar aturan ZUSTAND_GUIDE untuk komponen tampilan.
 */
import { Image } from 'expo-image';
import { View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useAvatarSource } from '@/hooks/use-avatar-source';
import { useAvatarStore } from '@/store/avatarStore';
import { PersonIcon } from '@/components/ui/Icon';

/**
 * Foto profil user yang login dalam lingkaran; tanpa foto (atau selama dimuat) tampil ikon orang.
 * `size` = sisi lingkaran, `iconSize` = ukuran ikon pengganti. Latar mengikuti tombol ikon yang diganti.
 */
export function UserAvatar({
  size,
  iconSize,
  background = false,
}: {
  /** Sisi lingkaran dalam piksel. */
  size: number;
  /** Ukuran ikon orang dalam piksel saat foto belum ada. */
  iconSize: number;
  /** true = lingkaran gelap di belakang ikon (panel); false = ikon polos (header). Default false. */
  background?: boolean;
}) {
  // updatedAt berubah saat foto diganti; hook memakainya agar sumber foto dihitung ulang.
  const updatedAt = useAvatarStore((s) => s.updatedAt);
  const source = useAvatarSource(updatedAt);

  if (source) {
    return (
      <View style={{ width: size, height: size }} className="overflow-hidden rounded-full bg-dark">
        <Image source={source} style={{ width: size, height: size }} contentFit="cover" accessibilityLabel="Foto profil" />
      </View>
    );
  }
  if (background) {
    return (
      <View style={{ width: size, height: size }} className="items-center justify-center rounded-full bg-dark">
        <PersonIcon size={iconSize} color={AppColors.white} />
      </View>
    );
  }
  return <PersonIcon size={iconSize} color={AppColors.white} />;
}
