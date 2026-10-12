import History from '@/components/History';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function HistoryRoute() {
  // TODO(developer): ambil riwayat queue (belum ada endpoint di backend).
  return <History items={[]} onMenuItemPress={handleOwnerMenuItemPress} />;
}
