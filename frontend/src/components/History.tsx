/**
 * History: daftar antrian yang sudah ditutup (riwayat) milik owner, dengan paginasi.
 *
 * Dipakai oleh : app/(owner)/history/index.tsx
 * Bergantung   : layout/ResponsiveScreenLayout, ui/AppButton, ui/EmptyState, ui/Icon, ui/Toast, hooks/use-theme-mode, hooks/use-is-wide, utils/history, utils/format
 * Peran        : owner
 * Catatan      : hanya menampilkan antrian yang sudah ditutup; antrian aktif tidak pernah masuk riwayat.
 */
import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { useThemeColor } from '@/hooks/use-theme-mode';
import { useIsWide } from '@/hooks/use-is-wide';
import type { HistoryQueue } from '@/types/api';
import { cn } from '@/utils/cn';
import { formatDateTime } from '@/utils/format';
import { summarizeHistoryQueue } from '@/utils/history';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { EmptyState } from '@/components/ui/EmptyState';
import { Icon } from '@/components/ui/Icon';
import { useErrorToast } from '@/components/ui/Toast';

export type HistoryProps = {
  /** Konteks shell layout; hanya `userName` yang dipakai untuk header. Opsional. */
  shell?: ShellProps;
  /** Dipanggil saat item menu owner ditekan; opsional, tanpa nilai menu tidak ada aksi. */
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** Antrian yang sudah ditutup, terbaru dulu. Antrian aktif tidak pernah masuk riwayat. */
  items: HistoryQueue[];
  /** true saat data pertama sedang dimuat; teks "Memuat riwayat..." tampil. Default false. */
  loading?: boolean;
  /** true bila masih ada halaman berikutnya; tombol "Muat lagi" ditampilkan. Default false. */
  hasMore?: boolean;
  /** true saat halaman berikutnya sedang dimuat; hanya tombol "Muat lagi" yang berstatus loading. Default false. */
  loadingMore?: boolean;
  /** Dipanggil saat tombol "Muat lagi" ditekan untuk mengambil halaman berikutnya. Opsional. */
  onLoadMore?: () => void;
  /** Galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
};

/** Halaman riwayat antrian owner: kartu per antrian yang sudah ditutup, dengan tautan ke detailnya. */
export default function History({
  shell,
  onMenuItemPress,
  items,
  loading = false,
  hasMore = false,
  loadingMore = false,
  onLoadMore,
  errorMessage,
}: HistoryProps) {
  useErrorToast(errorMessage);
  const avatarColor = useThemeColor('avatar');
  const foreground = useThemeColor('foreground');
  const isWide = useIsWide();
  const empty = items.length === 0;

  return (
    <ResponsiveScreenLayout
      title="Riwayat Antrian"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="history"
      onBack={() => router.back()}>
      {loading ? <Text className="text-[13px] text-muted">Memuat riwayat...</Text> : null}

      {empty && !loading ? (
        <EmptyState
          icon={<Icon name="history" size={44} color={avatarColor} />}
          title="Belum Ada Riwayat"
          description="Antrian yang sudah ditutup akan muncul di sini."
        />
      ) : (
        <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
          {items.map((queue) => (
            <View
              key={queue.id}
              className={cn(
                'flex-row items-center rounded-card border border-line p-3.5',
                isWide && 'w-[31.5%]',
              )}>
              <View className="flex-1 gap-[3px]">
                <Text className="text-[15px] font-extrabold text-foreground">{queue.queue_name}</Text>
                <Text className="text-[12px] font-normal text-muted">Ditutup {formatDateTime(queue.closed_at)}</Text>
                <Text className="text-[12px] font-normal text-muted">{summarizeHistoryQueue(queue)}</Text>
              </View>
              <Pressable
                accessibilityRole="button"
                accessibilityLabel={`Lihat riwayat ${queue.queue_name}`}
                className="h-10 w-10 items-center justify-center rounded-[10px] bg-surface"
                onPress={() =>
                  router.push({ pathname: '/history/[queueId]', params: { queueId: queue.id } })
                }>
                <Icon name="search" size={20} color={foreground} />
              </Pressable>
            </View>
          ))}
        </View>
      )}

      {hasMore ? (
        <AppButton label="Muat lagi" variant="outline" loading={loadingMore} onPress={onLoadMore} />
      ) : null}
    </ResponsiveScreenLayout>
  );
}
