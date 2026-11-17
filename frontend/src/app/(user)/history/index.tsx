/**
 * history/index: riwayat antrian owner: daftar antrian yang sudah ditutup, dimuat per halaman.
 *
 * Rute        : /history.
 * Akses       : butuh login (AuthGate dari (user)/_layout.tsx); data hanya milik owner yang masuk.
 * Dipakai oleh: menu panel (case history di utils/menu).
 * Bergantung  : useHistoryQueues (lib/history), History.
 * Catatan     : Tap antrian membuka /history/[queueId].
 */
import History from '@/components/History';
import { useHistoryQueues } from '@/hooks/use-history';
import { useAuthStore } from '@/store/authStore';
import { handleMenuItemPress } from '@/utils/menu';

/** Riwayat Antrian: antrian yang sudah ditutup, dimuat dari endpoint riwayat per halaman. */
export default function HistoryRoute() {
  const history = useHistoryQueues();
  const displayName = useAuthStore((s) => s.displayName);

  return (
    <History
    shell = {{userName: displayName ?? undefined}}
      items={history.items}
      loading={history.loading}
      hasMore={history.hasMore}
      loadingMore={history.loadingMore}
      onLoadMore={() => void history.loadMore()}
      errorMessage={history.error}
      onMenuItemPress={handleMenuItemPress}
    />
  );
}
