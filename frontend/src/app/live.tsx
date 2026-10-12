import QueueLiveScreen from '@/components/QueueLiveScreen';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

// Route pratinjau sementara untuk mengecek tampilan QueueLiveScreen dengan data statis.
// TODO(developer): hapus setelah tampilan dikonfirmasi; route sebenarnya ada di /queues/[queueId]/live.
export default function QueueLivePreviewRoute() {
  return (
    <QueueLiveScreen
      queueName="Antrian Loket 1"
      counterName="Counter 2"
      previousTicket={{ number: 'A-041', name: 'Budi Santoso' }}
      currentTicket={{ number: 'A-042', name: 'Siti Rahma' }}
      nextTicket={{ number: 'A-043', name: 'Andi Wijaya' }}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
