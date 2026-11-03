import { router, useLocalSearchParams } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';
import { CustomerQueueScreen, type CustomerTicketStatus } from '@/components/CustomerQueueScreen';
import { useTicketQueue } from '@/hooks/use-ticket-queue';
import { canCancelTicket } from '@/lib/tickets';
import { formatTicketNumber } from '@/utils/format';

/**
 * Status tiket saya di satu antrian (dibuka dari "Antrian Saya" / "Riwayat").
 * Join baru hanya lewat token QR di `/join?t=...`, jadi halaman ini tidak punya tombol join.
 */
export default function TicketQueueRoute() {
  return (
    <AuthGate mode="required">
      <TicketQueueContent />
    </AuthGate>
  );
}

function TicketQueueContent() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const id = Number(queueId);
  const { status, loading, error, cancel, cancelling, cancelError } = useTicketQueue(id);

  const queue = status?.queue;
  const ticket = status?.my_ticket ?? null;
  const closed = queue?.status === 'closed';
  // Customer hanya bisa membatalkan nomor yang masih aktif di antrian yang masih buka (4.9).
  const canCancel = canCancelTicket(ticket, closed);

  return (
    <CustomerQueueScreen
      queueName={queue?.queue_name ?? ''}
      queueDescription={queue?.queue_desc ?? null}
      ticketNumber={ticket ? formatTicketNumber(ticket.ticket_number, queue?.ticket_prefix) : null}
      ticketStatus={(ticket?.status as CustomerTicketStatus | undefined) ?? null}
      servingNow={ticket?.status === 'currently_served' && !closed}
      currentNumber={status?.current_number != null ? formatTicketNumber(status.current_number, queue?.ticket_prefix) : null}
      counterName={status?.current_counter_name ?? null}
      waitingAhead={status?.waiting_ahead ?? null}
      closed={closed}
      loading={loading}
      errorMessage={error ?? cancelError}
      canJoin={false}
      canCancel={canCancel}
      cancelling={cancelling}
      onCancel={() => void cancel()}
      onBack={() => router.replace('/dashboard')}
    />
  );
}
