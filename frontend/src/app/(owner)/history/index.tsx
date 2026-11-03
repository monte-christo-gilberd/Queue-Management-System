import History from '@/components/History';
import { useHistoryQueues } from '@/hooks/use-history';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Riwayat Antrian: antrian yang sudah ditutup, dimuat dari endpoint riwayat per halaman. */
export default function HistoryRoute() {
  const history = useHistoryQueues();

  return (
    <History
      items={history.items}
      loading={history.loading}
      hasMore={history.hasMore}
      loadingMore={history.loadingMore}
      onLoadMore={() => void history.loadMore()}
      errorMessage={history.error}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
