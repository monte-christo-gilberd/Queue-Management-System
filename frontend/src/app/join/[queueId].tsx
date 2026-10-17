import { router, useLocalSearchParams } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';
import { CustomerQueueScreen, type CustomerTicketStatus } from '@/components/CustomerQueueScreen';
import { useCustomerQueue } from '@/hooks/use-customer-queue';
import { formatTicketNumber } from '@/utils/format';

/**
 * Halaman join customer: `/join/<queueId>`. Harus login; belum login → `/?next=/join/<id>`.
 */
export default function JoinQueueRoute() {
  return (
    <AuthGate mode="required">
      <JoinQueueContent />
    </AuthGate>
  );
}

function JoinQueueContent() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const id = Number(queueId);
  const { status, loading, error, joining, joinError, join } = useCustomerQueue(id);

  const queue = status?.queue;
  const ticket = status?.my_ticket ?? null;
  const closed = queue?.status === 'closed';
  const canJoin = !ticket && !closed && !loading && !error;

  return (
    <CustomerQueueScreen
      queueName={queue?.queue_name ?? ''}
      ticketNumber={ticket ? formatTicketNumber(ticket.ticket_number) : null}
      ticketStatus={(ticket?.status as CustomerTicketStatus | undefined) ?? null}
      currentNumber={status?.current_number != null ? formatTicketNumber(status.current_number) : null}
      waitingAhead={status?.waiting_ahead ?? null}
      closed={closed}
      loading={loading}
      errorMessage={joinError ?? error}
      canJoin={canJoin}
      joining={joining}
      onJoin={() => void join()}
      onBack={() => router.replace('/dashboard')}
    />
  );
}
