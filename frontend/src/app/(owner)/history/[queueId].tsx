import HistoryDetail from '@/components/HistoryDetail';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function HistoryDetailRoute() {
  // TODO(developer): useLocalSearchParams() untuk queueId + ambil tiket riwayat (belum ada endpoint).
  return <HistoryDetail tickets={[]} onMenuItemPress={handleOwnerMenuItemPress} />;
}
