import { router, useLocalSearchParams } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';
import { CustomerQueueScreen, type CustomerTicketStatus } from '@/components/CustomerQueueScreen';
import { useCustomerQueue } from '@/hooks/use-customer-queue';
import { formatTicketNumber } from '@/utils/format';

/**
 * Halaman join customer berbasis token: `/join?t=<token>`.
 *
 * Token berasal dari QR statis/dinamik (dibuat qr-service, disimpan backend). Wajib login;
 * bila belum, AuthGate mengarahkan ke `/?next=%2Fjoin%3Ft%3D...` sehingga token tetap utuh.
 */
export default function JoinQueueRoute() {
  const { t } = useLocalSearchParams<{ t?: string }>();
  const token = typeof t === 'string' ? t : Array.isArray(t) ? t[0] : '';
  // usePathname() membuang query, jadi tujuan setelah login dikirim eksplisit.
  const nextPath = token ? `/join?t=${encodeURIComponent(token)}` : '/join';

  return (
    <AuthGate mode="required" nextPath={nextPath}>
      <JoinQueueContent token={token} />
    </AuthGate>
  );
}

function JoinQueueContent({ token }: { token: string }) {
  const { status, info, loading, error, joining, joinError, join } = useCustomerQueue(token);

  const queue = status?.queue ?? info;
  const ticket = status?.my_ticket ?? null;
  const closed = queue?.status === 'closed';
  const canJoin = !!token && !ticket && !closed && !loading && !error;

  return (
    <CustomerQueueScreen
      queueName={queue?.queue_name ?? ''}
      queueDescription={queue?.queue_desc ?? null}
      ticketNumber={ticket ? formatTicketNumber(ticket.ticket_number) : null}
      ticketStatus={(ticket?.status as CustomerTicketStatus | undefined) ?? null}
      // Sedang dilayani hanya selama antrian masih buka; setelah ditutup, nomor terakhir jadi "Sudah dilayani".
      servingNow={ticket != null && !closed && status?.current_number === ticket.ticket_number}
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
