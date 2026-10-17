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
  name: string;
  email: string;
  password: string;
  /** Isi field konfirmasi password (untuk mengecek kesamaan dengan `password`). */
  confirmPassword: string;
  onChangeName: (value: string) => void;
  onChangeEmail: (value: string) => void;
  onChangePassword: (value: string) => void;
  onChangeConfirmPassword: (value: string) => void;
  /** Pesan error di bawah field konfirmasi (mis. "Password tidak sama"); logika pengecekannya di route. */
  confirmPasswordError?: string;
  /** TODO: backend belum mendukung ini (upload foto profil) */
  onPressAvatar?: () => void;
  onSubmit: () => void;
  loading: boolean;
  errorMessage?: string;
  /** Status subscription; implementasi pengecekan belum ada → default dianggap belum bayar. TODO: backend belum mendukung ini */
  subscriptionPaid?: boolean;
  /** Keterangan di bawah status, mis. "Aktif hingga 01 Feb 2025". */
  subscriptionDescription?: string;
};

export default function Profile({
  shell,
  onMenuItemPress,
  name,
  email,
  password,
  confirmPassword,
  onChangeName,
  onChangeEmail,
  onChangePassword,
  onChangeConfirmPassword,
  confirmPasswordError,
  onPressAvatar,
  onSubmit,
  loading,
  errorMessage,
  subscriptionPaid = false,
  subscriptionDescription,
}: ProfileProps) {
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Profil"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="profile"
      profileHighlighted
      onBack={() => router.back()}>
      <View className={isWide ? 'flex-row items-start gap-6' : 'gap-4'}>
        <View className={cn('gap-4', isWide && 'w-80')}>
          <Pressable
            className={cn(
              'items-center gap-2.5 rounded-card bg-surface',
              isWide ? 'w-80 py-8' : 'py-3.5',
            )}
            onPress={onPressAvatar}>
            <View className="h-[88px] w-[88px] items-center justify-center rounded-full bg-dark">
              <PersonIcon size={40} color={AppColors.white} />
            </View>
            <Text className="text-[12px] font-normal text-muted">Klik untuk mengubah foto profil</Text>
          </Pressable>

          {/* Status subscription: link ke halaman Subscription hanya muncul kalau belum bayar. */}
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
        </View>

        {/* TODO: backend belum mendukung ini (edit profil) */}
        <View className={cn('gap-4', isWide && 'flex-1')}>
          <FormField label="Nama User" value={name} onChangeText={onChangeName} />
          <FormField
            label="Email User"
            value={email}
            onChangeText={onChangeEmail}
            autoCapitalize="none"
            keyboardType="email-address"
          />
          <FormField
            label="Password"
            value={password}
            onChangeText={onChangePassword}
            secureTextEntry
          />
          <View className="gap-1.5">
            <FormField
              label="Konfirmasi Password"
              value={confirmPassword}
              onChangeText={onChangeConfirmPassword}
              secureTextEntry
            />
            {confirmPasswordError ? (
              <Text className="text-[12px] font-normal text-live">{confirmPasswordError}</Text>
            ) : null}
          </View>

          {errorMessage ? <Text className="text-[12px] font-normal text-live">{errorMessage}</Text> : null}

          <AppButton label="Simpan Perubahan" variant="dark" onPress={onSubmit} loading={loading} />
        </View>
      </View>
    </ResponsiveScreenLayout>
  );
}
