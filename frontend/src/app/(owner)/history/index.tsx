import History from '@/components/History';
import { useQueueList } from '@/hooks/use-queues';
import type { HistoryItem } from '@/types/api';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Riwayat Antrian: antrian yang sudah ditutup, dari daftar antrian owner. */
export default function HistoryRoute() {
  const { queues } = useQueueList();

  // Nama admin per antrian belum ada di backend, jadi kosong.
  const items: HistoryItem[] = queues
    .filter((q) => q.status === 'closed')
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
    .map((queue) => ({ queue, adminNames: [] }));

  return <History items={items} onMenuItemPress={handleOwnerMenuItemPress} />;
}
