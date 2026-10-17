import { useState } from 'react';

import QueueList, { type QueueListFilter } from '@/components/QueueList';
import { useQueueList } from '@/hooks/use-queues';
import { useAuthStore } from '@/store/authStore';
import type { QueueListItem } from '@/types/api';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueListRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const [filter, setFilter] = useState<QueueListFilter>('all');
  const { queues, loading, error } = useQueueList();

  // Filter dilakukan di klien; backend sudah membatasi daftar bila langganan tidak aktif.
  const visible = filter === 'active' ? queues.filter((q) => q.status === 'active') : queues;
  const items: QueueListItem[] = visible.map((queue) => ({ queue }));

  return (
    <QueueList
      shell={{ userName: displayName ?? undefined }}
      items={items}
      filter={filter}
      onChangeFilter={setFilter}
      loading={loading}
      errorMessage={error}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
