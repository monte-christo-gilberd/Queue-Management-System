import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Text } from 'react-native';

import QueueDetail from '@/components/QueueDetail';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { useQueueDetail } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { closeQueue, updateQueue } from '@/lib/queues';
import { useAuthStore } from '@/store/authStore';
import type { Queue } from '@/types/api';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueDetailRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const displayName = useAuthStore((s) => s.displayName);
  const { queue, loading, error, reload } = useQueueDetail(Number(queueId));

  if (!queue) {
    return (
      <ResponsiveScreenLayout
        title="Detail Antrian"
        userName={displayName ?? undefined}
        onMenuItemPress={handleOwnerMenuItemPress}
        activeItem="queues"
        activeTab="queues"
        onBack={() => router.back()}
      >
        <Text className="text-[13px] text-muted">{loading ? 'Memuat antrian...' : error}</Text>
      </ResponsiveScreenLayout>
    );
  }

  // key per antrian: form diisi ulang dari data server saat berpindah antrian.
  return (
    <QueueDetailEditor
      key={queue.id}
      queue={queue}
      reload={reload}
      userName={displayName ?? undefined}
    />
  );
}

/** Menyimpan isian form (state tampilan) dan memanggil PUT/POST. Dipisah agar form ikut key antrian. */
function QueueDetailEditor({
  queue,
  reload,
  userName,
}: {
  queue: Queue;
  reload: () => Promise<void>;
  userName?: string;
}) {
  const [queueName, setQueueName] = useState(queue.queue_name);
  const [queueDescription, setQueueDescription] = useState(queue.queue_desc ?? '');
  const [saving, setSaving] = useState(false);
  const [closing, setClosing] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  async function save() {
    const name = queueName.trim();
    if (!name) {
      setErrorMessage('Nama antrian wajib diisi');
      return;
    }
    setSaving(true);
    setErrorMessage(undefined);
    try {
      await updateQueue(queue.id, { queue_name: name, queue_description: queueDescription.trim() });
      await reload();
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setSaving(false);
    }
  }

  async function close() {
    setClosing(true);
    setErrorMessage(undefined);
    try {
      await closeQueue(queue.id);
      await reload();
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setClosing(false);
    }
  }

  return (
    <QueueDetail
      shell={{ userName }}
      onMenuItemPress={handleOwnerMenuItemPress}
      queue={queue}
      current={null}
      queueName={queueName}
      queueDescription={queueDescription}
      onChangeQueueName={setQueueName}
      onChangeQueueDescription={setQueueDescription}
      onSave={save}
      saving={saving}
      onCloseQueue={close}
      closing={closing}
      errorMessage={errorMessage}
    />
  );
}
