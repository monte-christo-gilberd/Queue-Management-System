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

export type SubscriptionProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  paid: boolean;
  /** Judul tagihan apa adanya, mis. "Tagihan Bulan 01/2025 Belum Dibayar". */
  title: string;
  /** Subjudul, mis. "Lakukan pembayaran sebelum 01 Feb 2025". */
  subtitle: string;
  username: string;
  /** Nilai yang di-encode ke QR pembayaran. */
  qrValue?: string;
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
  qrValue,
  onRefresh,
  refreshing = false,
}: SubscriptionProps) {
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Subscription"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="home"
      onBack={() => router.back()}>
      {/* TODO: backend belum mendukung ini (subscription & pembayaran) */}
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
            <Text className="text-[15px] font-extrabold text-foreground">QR Pembayaran</Text>
            {/* TODO: render QR dari `qrValue` pakai library QR client-side (belum ada di dependency) */}
            <View
              className="h-[120px] w-[120px] rounded-lg border border-line bg-white"
              accessibilityLabel={qrValue}
            />
            <Text className="text-[11px] text-muted">Username: {username}</Text>
          </View>

          {paid ? null : (
            <AppButton
              label="REFRESH"
              variant="dark"
              icon={<Icon name="refresh" size={20} color={AppColors.white} />}
              onPress={onRefresh}
              loading={refreshing}
            />
          )}
        </View>
      </View>
    </ResponsiveScreenLayout>
  );
}
