import { useState } from 'react';

import QueueList, { type QueueListFilter } from '@/components/QueueList';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueListRoute() {
  const [filter, setFilter] = useState<QueueListFilter>('all');
  // TODO(developer): GET /queues, lalu filter sesuai `filter`.

  return (
    <QueueList
      items={[]}
      filter={filter}
      onChangeFilter={setFilter}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
