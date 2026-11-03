import { useLocalSearchParams } from 'expo-router';

import HistoryDetail from '@/components/HistoryDetail';
import { useHistoryQueue } from '@/hooks/use-history';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Detail riwayat satu antrian yang sudah ditutup: semua nomor, customer, hasil akhir, dan catatan owner. */
export default function HistoryDetailRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const id = Number(queueId);
  const validId = Number.isInteger(id) && id > 0 ? id : null;
  const history = useHistoryQueue(validId);

  return (
    <HistoryDetail
      queue={history.queue}
      tickets={history.tickets}
      loading={history.loading}
      hasMore={history.hasMore}
      loadingMore={history.loadingMore}
      onLoadMore={() => void history.loadMore()}
      errorMessage={history.error}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
