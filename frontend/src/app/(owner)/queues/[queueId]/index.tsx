import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Text } from 'react-native';

import QueueDetail from '@/components/QueueDetail';
import { QueueQRCard } from '@/components/QueueQRCard';
import type { TimelineItem } from '@/components/QueueTimeline';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useTicketNote } from '@/hooks/use-ticket-note';
import { AppendNoteModal } from '@/components/ui/AppendNoteModal';
import { useQueueQR } from '@/hooks/use-queue-qr';
import { useSkipSettings } from '@/hooks/use-skip-settings';
import { SkipSettingsCard } from '@/components/SkipSettingsCard';
import { useQueueDetail } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { closeQueue, updateQueue } from '@/lib/queues';
import { useAuthStore } from '@/store/authStore';
import type { Queue } from '@/types/api';
import { formatTicketNumber } from '@/utils/format';
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

/** Menyimpan isian form (state tampilan), memuat papan tiket, dan memanggil PUT/POST. Dipisah agar form ikut key antrian. */
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
  const [ticketPrefix, setTicketPrefix] = useState(queue.ticket_prefix ?? 'A');
  const [saving, setSaving] = useState(false);
  const [closing, setClosing] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  // QR antrian: token statis/dinamis + countdown + penggantian kode (qr-service lewat backend).
  const qr = useQueueQR(queue.id);
  const skipCfg = useSkipSettings(queue.id);

  // Papan tiket: daftar tiket + SKIP/NEXT. Antrian ditutup dari sini memuat ulang detail.
  const board = useQueueBoard(queue.id, () => void reload());
  const currentId = board.view.current?.ticket.id ?? null;
  const timeline: TimelineItem[] = board.tickets.flatMap((t): TimelineItem[] => {
    // Tiket yang dihapus owner tidak ikut ditampilkan di timeline.
    if (t.status === 'deleted') return [];
    return [{
      number: formatTicketNumber(t.ticket_number, queue.ticket_prefix),
      status: t.id === currentId || t.status === 'currently_served' ? 'current' : t.status === 'completed' ? 'done' : t.status,
    }];
  });
  // Catatan menempel ke nomor yang sedang dilayani; tanpa nomor itu, tombol catatan nonaktif.
  const note = useTicketNote(queue.id);
  const currentTicketId = board.view.current?.ticket.id ?? null;
  const currentNumber = board.view.current ? formatTicketNumber(board.view.current.ticket.ticket_number, queue.ticket_prefix) : null;

  async function save() {
    const name = queueName.trim();
    if (!name) {
      setErrorMessage('Nama antrian wajib diisi');
      return;
    }
    setSaving(true);
    setErrorMessage(undefined);
    try {
      await updateQueue(queue.id, { queue_name: name, queue_description: queueDescription.trim(), ticket_prefix: ticketPrefix || undefined });
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
      // Setelah ditutup, owner kembali ke Dashboard (antrian itu tidak lagi tampil di sana).
      // navigate, bukan replace: Dashboard yang sudah ada dipakai lagi, tidak dibuat instance baru.
      router.navigate('/dashboard');
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setClosing(false);
    }
  }

  return (
    <>
    <QueueDetail
      shell={{ userName }}
      onMenuItemPress={handleOwnerMenuItemPress}
      queue={queue}
      queueName={queueName}
      queueDescription={queueDescription}
      onChangeQueueName={setQueueName}
      onChangeQueueDescription={setQueueDescription}
      ticketPrefix={ticketPrefix}
      onChangeTicketPrefix={setTicketPrefix}
      onSave={save}
      saving={saving}
      errorMessage={errorMessage ?? board.nextError ?? board.skipError ?? board.recallError ?? board.error}
      timeline={timeline}
      currentNumber={currentNumber}
      onSkip={() => void board.skip()}
      skipLoading={board.skipLoading}
      onNext={() => void board.next()}
      onRecall={() => void board.recall()}
      recallDisabled={!board.recallReady}
      recallLoading={board.recallLoading}
      showRecall={board.showRecall}
      skipDisabled={!board.skipReady}
      nextLoading={board.nextLoading}
      qrCard={
        <QueueQRCard
          qr={qr.qr}
          loading={qr.loading}
          error={qr.error}
          toggling={qr.toggling}
          replacing={qr.replacing}
          countdown={qr.countdown}
          onToggle={(type) => void qr.toggle(type)}
          onReplace={() => void qr.replace()}
          onRefresh={qr.reload}
          onRetry={qr.refresh}
        />
      }
      skipSettingsCard={
        skipCfg.settings ? (
          <SkipSettingsCard
            settings={skipCfg.settings}
            disabled={queue.status === 'closed'}
            saving={skipCfg.saving}
            saved={skipCfg.saved}
            error={skipCfg.error}
            onSave={(v) => void skipCfg.save(v)}
          />
        ) : null
      }
      onCloseQueue={close}
      closing={closing}
      onOpenNote={currentTicketId !== null ? () => void note.open(currentTicketId) : undefined}
    />
    <AppendNoteModal
      visible={note.visible}
      onCancel={note.close}
      onSubmit={(text) => void note.save(text)}
      initialNote={note.initialNote}
      hasNote={note.hasNote}
      loading={note.loading}
      saving={note.saving}
      errorMessage={note.errorMessage}
    />
    </>
  );
}
