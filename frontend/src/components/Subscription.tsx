/**
 * Subscription: halaman status dan perpanjangan langganan owner.
 *
 * Dipakai oleh : app/(owner)/subscription.tsx
 * Bergantung   : layout/ResponsiveScreenLayout, ui/AppButton, ui/Icon, ui/Toast, constants/app-theme, utils/cn
 * Peran        : owner
 * Catatan      : MOCK. Tidak ada pembayaran; tombol langsung menambah 30 hari lewat backend (lihat lib/subscription.ts).
 */
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

/** Props halaman langganan owner. */
export type SubscriptionProps = {
    /** Konteks shell layout; hanya `userName` yang dipakai untuk header. Opsional. */
  shell?: ShellProps;
    /** Dipanggil saat item menu owner ditekan; opsional, tanpa nilai menu tidak ada aksi. */
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** true jika langganan aktif (kartu hijau dengan ikon centang); false menampilkan kartu abu dengan ikon jam. */
  paid: boolean;
  /** Judul tagihan apa adanya, mis. "Tagihan Bulan 01/2025 Belum Dibayar". */
  title: string;
  /** Subjudul, mis. "Lakukan pembayaran sebelum 01 Feb 2025". */
  subtitle: string;
  /** Username akun owner, hanya ditampilkan sebagai informasi di kartu pembayaran. */
  username: string;
  /** Tambah 30 hari langsung (MOCK, tanpa pembayaran). Untuk calon owner, 30 hari pertama menjadikannya owner. */
  onPay?: () => void;
  /** Judul kartu pembayaran. Opsional; default "Perpanjang 1 Bulan". */
  payTitle?: string;
  /** Penjelasan singkat di bawah judul kartu pembayaran. Opsional; default menyebut penambahan 30 hari. */
  payHint?: string;
  /** Teks tombol bayar. Opsional; default "PERPANJANG 1 BULAN". */
  payLabel?: string;
  /** true saat proses pembayaran berjalan; tombol bayar menampilkan status loading. Default false. */
  paying?: boolean;
  /** Info setelah kembali dari pembayaran (mis. "Pembayaran diterima..."). Opsional; tidak ditampilkan bila kosong. */
  message?: string;
  /** Galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
  /** Dipanggil saat tombol REFRESH ditekan untuk memuat ulang status langganan. Opsional. */
  onRefresh?: () => void;
  /** true saat refresh berjalan; tombol REFRESH menampilkan status loading. Default false. */
  refreshing?: boolean;
};

/** Kartu status langganan, kartu pembayaran, dan tombol REFRESH untuk owner. */
export default function Subscription({
  shell,
  onMenuItemPress,
  paid,
  title,
  subtitle,
  username,
  onPay,
  payTitle = 'Tambah 30 Hari',
  payHint = '[MOCK] Tanpa pembayaran. Masa langganan langsung bertambah 30 hari.',
  payLabel = 'TAMBAH 30 HARI',
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
