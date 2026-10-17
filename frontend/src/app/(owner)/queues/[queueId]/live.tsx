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

  return (
    <QueueLiveScreen
      queueName={queue?.queue_name ?? 'Antrian'}
      counterName="-"
      previousTicket={toLive(previous)}
      currentTicket={toLive(current)}
      nextTicket={toLive(next)}
      onNext={() => void board.next()}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}

function toLive(entry: TicketEntry | null) {
  return entry ? { number: formatTicketNumber(entry.ticket.ticket_number), name: entry.customerName } : null;
}
