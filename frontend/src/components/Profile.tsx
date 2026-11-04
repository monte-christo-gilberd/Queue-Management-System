import { useErrorToast } from '@/components/ui/Toast';
import { USERNAME_MAX } from '@/utils/profile';
import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useIsWide } from '@/hooks/use-is-wide';
import { cn } from '@/utils/cn';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { PersonIcon } from '@/components/ui/Icon';

export type ProfileProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  username: string;
  /** Email hanya ditampilkan; mengubahnya menunggu verifikasi lewat email (plan item 9). */
  email: string;
  onChangeUsername: (value: string) => void;
  onSubmit: () => void;
  /** Tombol simpan nonaktif bila nama tidak berubah atau tidak valid. */
  canSave: boolean;
  loading: boolean;
  errorMessage?: string;
  /** Pesan sukses di bawah tombol simpan (mis. "Perubahan tersimpan"). */
  savedMessage?: string;
  /** Kartu langganan hanya untuk owner. */
  isOwner: boolean;
  subscriptionPaid?: boolean;
  /** Keterangan di bawah status, mis. "Aktif hingga 1 Feb 2025". */
  subscriptionDescription?: string;
  /** Placeholder: belum ada fungsinya sampai alur ganti password lewat email jadi (plan item 9). */
  onPressChangePassword?: () => void;
};

export default function Profile({
  shell,
  onMenuItemPress,
  username,
  email,
  onChangeUsername,
  onSubmit,
  canSave,
  loading,
  errorMessage,
  savedMessage,
  isOwner,
  subscriptionPaid = false,
  subscriptionDescription,
  onPressChangePassword,
}: ProfileProps) {
  useErrorToast(errorMessage);
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Profil"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="profile">
      <View className={isWide ? 'flex-row items-start gap-6' : 'gap-4'}>
        <View className={cn('gap-4', isWide && 'w-80')}>
          {/* Foto profil di luar cakupan (butuh penyimpanan berkas): hanya ikon. */}
          <View
            className={cn(
              'items-center gap-2.5 rounded-card bg-surface',
              isWide ? 'w-80 py-8' : 'py-3.5',
            )}>
            <View className="h-[88px] w-[88px] items-center justify-center rounded-full bg-dark">
              <PersonIcon size={40} color={AppColors.white} />
            </View>
          </View>

          {/* Status subscription (owner): link ke halaman Subscription hanya muncul kalau belum bayar. */}
          {isOwner ? (
            <View className="gap-2 rounded-card border border-line p-4">
              <View className="flex-row items-center justify-between">
                <Text className="text-[14px] font-bold text-foreground">Status Subscription</Text>
                <View
                  className={cn(
                    'rounded-pill px-3 py-1',
                    subscriptionPaid ? 'bg-primary' : 'bg-disabled',
                  )}>
                  <Text
                    className={cn(
                      'text-[11px] font-extrabold',
                      subscriptionPaid ? 'text-white' : 'text-foreground',
                    )}>
                    {subscriptionPaid ? 'Aktif' : 'Belum Bayar'}
                  </Text>
                </View>
              </View>
              <Text className="text-[12px] font-normal text-muted">
                {subscriptionDescription ??
                  (subscriptionPaid
                    ? 'Langganan Anda aktif.'
                    : 'Anda belum berlangganan atau belum membayar tagihan bulan ini.')}
              </Text>
              {subscriptionPaid ? null : (
                <Pressable onPress={() => router.push('/subscription')} accessibilityRole="link">
                  <Text className="text-[13px] font-bold text-primary">Lihat Subscription & Bayar →</Text>
                </Pressable>
              )}
            </View>
          ) : null}
        </View>

        <View className={cn('gap-4', isWide && 'flex-1')}>
          <FormField
            label="Nama User"
            value={username}
            onChangeText={onChangeUsername}
            placeholder="Nama User"
            maxLength={USERNAME_MAX}
          />
          <FormField label="Email User" value={email} editable={false} placeholder="Email User" />

          <AppButton
            label="Simpan Perubahan"
            variant="dark"
            onPress={onSubmit}
            loading={loading}
            disabled={!canSave}
          />
          {savedMessage ? (
            <Text className="text-[13px] font-semibold text-primary">{savedMessage}</Text>
          ) : null}

          <View className="gap-2 rounded-card border border-line p-4">
            <Text className="text-[14px] font-bold text-foreground">Password</Text>
            <Text className="text-[12px] font-normal text-muted">
              Tautan untuk membuat password baru akan dikirim ke email akun Anda.
            </Text>
            <AppButton label="Minta Ganti Password" variant="outline" onPress={onPressChangePassword} />
          </View>
        </View>
      </View>
    </ResponsiveScreenLayout>
  );
}
