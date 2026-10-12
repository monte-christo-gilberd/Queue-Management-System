import QueueLiveScreen from '@/components/QueueLiveScreen';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueLiveRoute() {
  return (
    <>
      {/* TODO: sambungkan previousTicket/currentTicket/nextTicket dari SSE, dan onSkip/onNext dari fetch, setelah tampilan dikonfirmasi */}
      <QueueLiveScreen onMenuItemPress={handleOwnerMenuItemPress} />
    </>
  );
}
