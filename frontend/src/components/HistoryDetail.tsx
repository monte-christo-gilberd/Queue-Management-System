import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { useIsWide } from '@/hooks/use-is-wide';
import type { Queue, Ticket, TicketEntry } from '@/types/api';
import { cn } from '@/utils/cn';
import { formatDate, formatTicketNumber } from '@/utils/format';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';

export type HistoryDetailProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  queue?: Queue | null;
  /** TODO: backend belum mendukung ini (belum ada endpoint history) */
  tickets: TicketEntry[];
};

const STATUS_LABEL: Record<Ticket['status'], string> = {
  waiting: 'Menunggu',
  currently_served: 'Sedang dilayani',
  completed: 'Selesai',
  cancelled: 'Dibatalkan',
  skipped: 'Dilewati',
  deleted: 'Dihapus',
};

export default function HistoryDetail({
  shell,
  onMenuItemPress,
  queue,
  tickets,
}: HistoryDetailProps) {
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Detail Riwayat"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="history"
      onBack={() => router.back()}>
      <View>
        <Text className="text-[22px] font-black text-foreground">Semua Nomor Antrian</Text>
        <Text className="text-[11px] text-muted">Nama Queue / Tanggal Queue</Text>
        <Text className="text-[14px] font-extrabold text-primary">
          {queue ? `${queue.queue_name} · ${formatDate(queue.created_at)}` : '-'}
        </Text>
      </View>

      <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
        {tickets.map(({ ticket, counterName }) => (
          <View
            key={ticket.id}
            className={cn(
              'flex-row items-center gap-2.5 rounded-card border border-line px-3.5 py-4',
              isWide && 'w-[48.5%]',
            )}>
            <Text className="text-[17px] font-black text-foreground">{formatTicketNumber(ticket.ticket_number)}</Text>
            {/* TODO: backend belum mendukung ini (counter/loket) */}
            <Text className="flex-1 text-[12px] font-normal text-muted">Nama Counter: {counterName ?? '-'}</Text>
            <View className="rounded-pill bg-badgeDone px-2.5 py-1">
              <Text className="text-[10px] font-extrabold text-dark">{STATUS_LABEL[ticket.status]}</Text>
            </View>
          </View>
        ))}
      </View>
    </ResponsiveScreenLayout>
  );
}
