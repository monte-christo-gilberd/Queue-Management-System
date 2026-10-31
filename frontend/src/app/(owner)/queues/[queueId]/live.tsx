import { useLocalSearchParams } from 'expo-router';

import QueueLiveScreen from '@/components/QueueLiveScreen';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useQueueDetail } from '@/hooks/use-queues';
import type { TicketEntry } from '@/types/api';
import { formatTicketNumber } from '@/utils/format';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueLiveRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const id = Number(queueId);
  const validId = Number.isInteger(id) && id > 0 ? id : null;

  const { queue } = useQueueDetail(id);
  const board = useQueueBoard(validId);
  const { previous, current, next } = board.view;
  // Nama counter hanya ditampilkan bila counter non-default memanggil (default disembunyikan backend).
  const counterName = current?.counterName ?? null;

  return (
    <QueueLiveScreen
      queueName={queue?.queue_name ?? 'Antrian'}
      counterName={counterName ?? undefined}
      previousTicket={toLive(previous, queue?.ticket_prefix)}
      currentTicket={toLive(current, queue?.ticket_prefix)}
      nextTicket={toLive(next, queue?.ticket_prefix)}
      onSkip={() => void board.skip()}
      onNext={() => void board.next()}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}

function toLive(entry: TicketEntry | null, prefix?: string) {
  return entry ? { number: formatTicketNumber(entry.ticket.ticket_number, prefix), name: entry.customerName } : null;
}
