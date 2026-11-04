import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useIsWide } from '@/hooks/use-is-wide';
import { cn } from '@/utils/cn';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { Icon } from '@/components/ui/Icon';
import { useErrorToast } from '@/components/ui/Toast';

export type SubscriptionProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  paid: boolean;
  /** Judul tagihan apa adanya, mis. "Tagihan Bulan 01/2025 Belum Dibayar". */
  title: string;
  /** Subjudul, mis. "Lakukan pembayaran sebelum 01 Feb 2025". */
  subtitle: string;
  username: string;
  /** Buka pembayaran Stripe untuk perpanjangan 1 bulan. */
  onPay?: () => void;
  paying?: boolean;
  /** Info setelah kembali dari pembayaran (mis. "Pembayaran diterima..."). */
  message?: string;
  errorMessage?: string;
  onRefresh?: () => void;
  refreshing?: boolean;
};

export default function Subscription({
  shell,
  onMenuItemPress,
  paid,
  title,
  subtitle,
  username,
  onPay,
  paying = false,
  message,
  errorMessage,
  onRefresh,
  refreshing = false,
}: SubscriptionProps) {
  const isWide = useIsWide();
  useErrorToast(errorMessage);

  return (
    <ResponsiveScreenLayout
      title="Subscription"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="home"
      onBack={() => router.back()}>
      <View className={isWide ? 'flex-row items-start gap-5' : 'gap-4'}>
        <View
          className={cn(
            'items-center justify-center gap-1.5 rounded-card p-6',
            isWide && 'flex-1',
            paid ? 'bg-primary' : 'bg-disabled',
          )}>
          <Icon
            name={paid ? 'check' : 'clock'}
            size={38}
            color={paid ? AppColors.white : AppColors.dark}
          />
          <Text
            className={cn(
              'text-center text-[18px] font-black',
              paid ? 'text-white' : 'text-foreground',
            )}>{title}</Text>
          <Text
            className={cn(
              'text-center text-[12px] font-normal',
              paid ? 'text-white' : 'text-muted',
            )}>{subtitle}</Text>
        </View>

        <View className={cn('gap-4', isWide && 'flex-1')}>
          <View className="items-center gap-2.5 rounded-card bg-surface p-4">
            <Text className="text-[15px] font-extrabold text-foreground">Perpanjang 1 Bulan</Text>
            <Text className="text-center text-[12px] text-muted">
              Pembayaran lewat Stripe. Setelah berhasil, masa langganan bertambah 1 bulan.
            </Text>
            <AppButton label="PERPANJANG 1 BULAN" variant="primary" onPress={onPay} loading={paying} className="self-stretch" />
            {message ? <Text className="text-center text-[12px] text-foreground">{message}</Text> : null}
            <Text className="text-[11px] text-muted">Username: {username}</Text>
          </View>

          <AppButton
            label="REFRESH"
            variant="dark"
            icon={<Icon name="refresh" size={20} color={AppColors.white} />}
            onPress={onRefresh}
            loading={refreshing}
          />
        </View>
      </View>
    </ResponsiveScreenLayout>
  );
}
