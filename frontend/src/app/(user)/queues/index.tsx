/**
 * queues/index: daftar antrian owner: hanya antrian yang masih aktif.
 *
 * Rute        : /queues.
 * Akses       : butuh login (AuthGate dari (user)/_layout.tsx); hanya owner yang punya antrian.
 * Dipakai oleh: menu panel (case queues di utils/menu).
 * Bergantung  : useQueueList, QueueList.
 * Catatan     : Antrian yang sudah ditutup ada di /history.
 */
import QueueList from '@/components/QueueList';
import { useQueueList } from '@/hooks/use-queues';
import { useAuthStore } from '@/store/authStore';
import type { QueueListItem } from '@/types/api';
import { handleMenuItemPress } from '@/utils/menu';

/** Daftar Antrian: hanya antrian yang masih aktif. Yang sudah ditutup ada di Riwayat Antrian. */
export default function QueueListRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const { queues, loading, error } = useQueueList();

  const items: QueueListItem[] = queues
    .filter((q) => q.status === 'active')
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
    .map((queue) => ({ queue }));

  return (
    <QueueList
      shell={{ userName: displayName ?? undefined }}
      items={items}
      loading={loading}
      errorMessage={error}
      onMenuItemPress={handleMenuItemPress}
    />
  );
}
