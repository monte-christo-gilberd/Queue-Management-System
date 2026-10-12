import { useState } from 'react';

import QueueDetail from '@/components/QueueDetail';
import type { Queue } from '@/types/api';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

// Placeholder agar kerangka bisa dirender — developer menggantinya dengan hasil GET /queues/:id.
const PLACEHOLDER_QUEUE: Queue = {
  id: 0,
  owner_id: 0,
  queue_name: '',
  queue_desc: null,
  status: 'active',
  created_at: new Date(0).toISOString(),
  closed_at: null,
};

export default function QueueDetailRoute() {
  const [queueName, setQueueName] = useState('');
  const [queueDescription, setQueueDescription] = useState('');
  // TODO(developer): useLocalSearchParams() untuk queueId + fetch detail queue.

  return (
    <QueueDetail
      queue={PLACEHOLDER_QUEUE}
      queueName={queueName}
      queueDescription={queueDescription}
      onChangeQueueName={setQueueName}
      onChangeQueueDescription={setQueueDescription}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
