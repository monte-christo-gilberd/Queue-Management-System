import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Text } from 'react-native';

import QueueDetail from '@/components/QueueDetail';
import type { TimelineItem } from '@/components/QueueTimeline';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useQueueDetail } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { buildJoinLink } from '@/lib/join-link';
import { closeQueue, updateAuthz, updateQueue } from '@/lib/queues';
import { copyToClipboard, currentWebOrigin } from '@/lib/web-origin';
import { useAuthStore } from '@/store/authStore';
import type { Queue, QueueAuthz } from '@/types/api';
import { formatTicketNumber } from '@/utils/format';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function QueueDetailRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const displayName = useAuthStore((s) => s.displayName);
  const { queue, authz, loading, error, reload } = useQueueDetail(Number(queueId));

  if (!queue) {
    return (
      <ResponsiveScreenLayout
        title="Detail Antrian"
        userName={displayName ?? undefined}
        onMenuItemPress={handleOwnerMenuItemPress}
        activeItem="queues"
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
      authz={authz}
      reload={reload}
      userName={displayName ?? undefined}
    />
  );
}

/** Menyimpan isian form (state tampilan), memuat papan tiket, dan memanggil PUT/POST. Dipisah agar form ikut key antrian. */
function QueueDetailEditor({
  queue,
  authz,
  reload,
  userName,
}: {
  queue: Queue;
  authz?: QueueAuthz;
  reload: () => Promise<void>;
  userName?: string;
}) {
  const [queueName, setQueueName] = useState(queue.queue_name);
  const [queueDescription, setQueueDescription] = useState(queue.queue_desc ?? '');
  const [authzDraft, setAuthzDraft] = useState<QueueAuthz | undefined>(authz);
  const [saving, setSaving] = useState(false);
  const [savingAuthz, setSavingAuthz] = useState(false);
  const [closing, setClosing] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  const [linkCopied, setLinkCopied] = useState(false);
  const joinLink = buildJoinLink(queue.id, currentWebOrigin());

  // Papan tiket: daftar tiket + SKIP/NEXT. Antrian ditutup dari sini memuat ulang detail.
  const board = useQueueBoard(queue.id, () => void reload());
  const currentId = board.view.current?.ticket.id ?? null;
  const timeline: TimelineItem[] = board.tickets.map((t): TimelineItem => ({
    number: formatTicketNumber(t.ticket_number),
    status: t.id === currentId ? 'current' : t.status === 'completed' ? 'done' : t.status,
  }));
  const currentNumber = board.view.current ? formatTicketNumber(board.view.current.ticket.ticket_number) : null;

  async function copyLink() {
    if (!joinLink) return;
    const ok = await copyToClipboard(joinLink);
    setLinkCopied(ok);
    if (!ok) setErrorMessage('Tidak dapat menyalin. Salin tautan secara manual.');
  }

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

  async function saveAuthz() {
    if (!authzDraft) return;
    setSavingAuthz(true);
    setErrorMessage(undefined);
    try {
      await updateAuthz(queue.id, authzDraft);
      await reload();
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setSavingAuthz(false);
    }
  }

  async function close() {
    setClosing(true);
    setErrorMessage(undefined);
    try {
      await closeQueue(queue.id);
      // Setelah ditutup, owner kembali ke Dashboard (antrian itu tidak lagi tampil di sana).
      router.replace('/dashboard');
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
      queueName={queueName}
      queueDescription={queueDescription}
      onChangeQueueName={setQueueName}
      onChangeQueueDescription={setQueueDescription}
      onSave={save}
      saving={saving}
      errorMessage={errorMessage ?? board.nextError ?? board.error}
      timeline={timeline}
      currentNumber={currentNumber}
      onNext={() => void board.next()}
      nextLoading={board.nextLoading}
      joinLink={joinLink}
      onCopyLink={() => void copyLink()}
      linkCopied={linkCopied}
      authz={authzDraft}
      onToggleAuthz={(key, value) =>
        setAuthzDraft((draft) => (draft ? { ...draft, [key]: value } : draft))
      }
      onSaveAuthz={() => void saveAuthz()}
      savingAuthz={savingAuthz}
      onCloseQueue={close}
      closing={closing}
    />
  );
}
