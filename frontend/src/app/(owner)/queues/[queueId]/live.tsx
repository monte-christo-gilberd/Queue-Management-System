import { useLocalSearchParams } from 'expo-router';

import QueueLiveScreen from '@/components/QueueLiveScreen';
import { useCounterSession } from '@/hooks/use-counter-session';
import { useNextSkipConfirm } from '@/hooks/use-next-skip-confirm';
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
  // Layar live tanpa tab: memakai counter yang dipilih perangkat ini (dari dashboard atau detail).
  const counters = useCounterSession(validId);
  const board = useQueueBoard(validId, undefined, counters.counterId);
  const { previous, current, next } = board.view;
  // Nama counter hanya ditampilkan bila counter non-default memanggil (default disembunyikan backend).
  const counterName = current?.counterName ?? null;
  const confirm = useNextSkipConfirm({
    current: current ? formatTicketNumber(current.ticket.ticket_number, queue?.ticket_prefix) : null,
    counterName: counters.counters.length > 1 ? counters.current?.name : undefined,
    onNext: board.next,
    onSkip: board.skip,
  });

  return (
    <>
    <QueueLiveScreen
      queueName={queue?.queue_name ?? 'Antrian'}
      counterName={counterName ?? undefined}
      previousTicket={toLive(previous, queue?.ticket_prefix)}
      currentTicket={toLive(current, queue?.ticket_prefix)}
      nextTicket={toLive(next, queue?.ticket_prefix)}
      onSkip={confirm.requestSkip}
      onNext={confirm.requestNext}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
    {confirm.modal}
    </>
  );
}

function toLive(entry: TicketEntry | null, prefix?: string) {
  return entry ? { number: formatTicketNumber(entry.ticket.ticket_number, prefix), name: entry.customerName } : null;
}
