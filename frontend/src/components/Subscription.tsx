import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
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
  /** Buka pembayaran Stripe untuk 1 bulan (perpanjangan, atau pembayaran pertama = jadi owner). */
  onPay?: () => void;
  /** Teks kartu dan tombol bayar; bawaan untuk perpanjangan. */
  payTitle?: string;
  payHint?: string;
  payLabel?: string;
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
  payTitle = 'Perpanjang 1 Bulan',
  payHint = 'Pembayaran lewat Stripe. Setelah berhasil, masa langganan bertambah 1 bulan.',
  payLabel = 'PERPANJANG 1 BULAN',
  paying = false,
  message,
  errorMessage,
  onRefresh,
  refreshing = false,
}: SubscriptionProps) {
  useErrorToast(errorMessage);

  return (
    <ResponsiveScreenLayout
      title="Subscription"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="home"
      onBack={() => router.back()}>
      {/* Kartu bertumpuk atas ke bawah di semua lebar layar; di layar lebar dibatasi agar tidak melar. */}
      <View className="w-full max-w-[720px] gap-4 self-center">
        <View
          className={cn(
            'items-center justify-center gap-1.5 rounded-card p-6',
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

        <View className="gap-4">
          <View className="items-center gap-2.5 rounded-card bg-surface p-4">
            <Text className="text-[15px] font-extrabold text-foreground">{payTitle}</Text>
            <Text className="text-center text-[12px] text-muted">{payHint}</Text>
            <AppButton label={payLabel} variant="primary" onPress={onPay} loading={paying} className="self-stretch" />
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
