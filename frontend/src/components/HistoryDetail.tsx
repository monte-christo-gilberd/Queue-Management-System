import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { useIsWide } from '@/hooks/use-is-wide';
import type { HistoryQueue, HistoryTicket } from '@/types/api';
import { cn } from '@/utils/cn';
import { formatDateTime, formatTicketNumber } from '@/utils/format';
import { outcomeLabel, summarizeHistoryQueue, ticketTimeline } from '@/utils/history';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { useErrorToast } from '@/components/ui/Toast';

export type HistoryDetailProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** Ringkasan antrian yang sudah ditutup; null selama dimuat atau bila tidak ditemukan. */
  queue?: HistoryQueue | null;
  tickets: HistoryTicket[];
  loading?: boolean;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  errorMessage?: string;
};

export default function HistoryDetail({
  shell,
  onMenuItemPress,
  queue,
  tickets,
  loading = false,
  hasMore = false,
  loadingMore = false,
  onLoadMore,
  errorMessage,
}: HistoryDetailProps) {
  useErrorToast(errorMessage);
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Detail Riwayat"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="history"
      onBack={() => router.back()}>
      <View className="gap-0.5">
        <Text className="text-[22px] font-black text-foreground">Semua Nomor Antrian</Text>
        {queue ? (
          <>
            <Text className="text-[14px] font-extrabold text-primary">{queue.queue_name}</Text>
            <Text className="text-[12px] text-muted">
              Dibuka {formatDateTime(queue.opened_at)} · Ditutup {formatDateTime(queue.closed_at)}
            </Text>
            <Text className="text-[12px] text-muted">{summarizeHistoryQueue(queue)}</Text>
          </>
        ) : (
          <Text className="text-[13px] text-muted">{loading ? 'Memuat...' : '-'}</Text>
        )}
      </View>

      {queue && tickets.length === 0 && !loading ? (
        <Text className="text-[13px] text-muted">Tidak ada nomor di antrian ini.</Text>
      ) : null}

      <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
        {tickets.map((t) => (
          <View
            key={t.ticket_id}
            className={cn('gap-1.5 rounded-card border border-line px-3.5 py-4', isWide && 'w-[48.5%]')}>
            <View className="flex-row items-center gap-2.5">
              <Text className="text-[17px] font-black text-foreground">
                {formatTicketNumber(t.ticket_number, queue?.ticket_prefix)}
              </Text>
              <View className="flex-1">
                <Text className="text-[13px] font-bold text-foreground">{t.customer_name}</Text>
                <Text className="text-[11px] font-normal text-muted">{t.customer_email}</Text>
              </View>
              <View className="rounded-pill bg-badgeDone px-2.5 py-1">
                <Text className="text-[10px] font-extrabold text-dark">{outcomeLabel(t.outcome)}</Text>
              </View>
            </View>
            {t.counter_name ? <Text className="text-[12px] font-normal text-muted">Counter: {t.counter_name}</Text> : null}
            <Text className="text-[12px] font-normal text-muted">{ticketTimeline(t).join(' · ')}</Text>
            {t.skip_count > 0 ? (
              <Text className="text-[12px] font-normal text-muted">Dilewati {t.skip_count}x</Text>
            ) : null}
            {t.note ? (
              <View className="rounded-card bg-surface p-2.5">
                <Text className="text-[12px] text-foreground">Catatan: {t.note}</Text>
              </View>
            ) : null}
          </View>
        ))}
      </View>

      {hasMore ? <AppButton label="Muat lagi" variant="outline" loading={loadingMore} onPress={onLoadMore} /> : null}
    </ResponsiveScreenLayout>
  );
}
