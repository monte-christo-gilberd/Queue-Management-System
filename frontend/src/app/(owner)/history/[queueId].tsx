/**
 * history/[queueId]: detail satu antrian yang sudah ditutup di riwayat owner: nomor, customer, hasil akhir, dan catatan.
 *
 * Rute        : /history/[queueId].
 * Akses       : butuh login (AuthGate dari (owner)/_layout.tsx).
 * Dipakai oleh: daftar riwayat antrian owner (History) saat sebuah baris dipilih.
 * Bergantung  : useHistoryQueue, HistoryDetail.
 * Catatan     : queueId yang bukan bilangan bulat positif dikirim sebagai null ke hook, bukan ditangani sebagai galat di sini.
 */
import { useLocalSearchParams } from 'expo-router';

import HistoryDetail from '@/components/HistoryDetail';
import { useHistoryQueue } from '@/hooks/use-history';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Detail riwayat satu antrian yang sudah ditutup: semua nomor, customer, hasil akhir, dan catatan owner. */
export default function HistoryDetailRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const id = Number(queueId);
  // Parameter URL selalu string; hanya bilangan bulat positif yang dianggap id antrian yang valid.
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
      onExport={(format) => void history.exportAs(format)}
      exporting={history.exporting}
      errorMessage={history.error}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
