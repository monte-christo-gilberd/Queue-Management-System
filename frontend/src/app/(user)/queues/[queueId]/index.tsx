/**
 * queues/[queueId]/index: detail satu antrian owner: ubah nama dan pengaturan, QR, layar display, SKIP/NEXT, dan tutup antrian.
 *
 * Rute        : /queues/[queueId].
 * Akses       : butuh login (AuthGate dari (user)/_layout.tsx); hanya owner.
 * Dipakai oleh: dashboard (tap papan atau setelah buat antrian) dan daftar antrian.
 * Bergantung  : useQueueDetail, useQueueBoard (SSE), useQueueQR, useDisplayConfig, useSkipSettings, QueueDetail.
 * Catatan     : Antrian yang sudah ditutup tidak menawarkan hapus nomor. Tutup antrian mengembalikan owner ke /dashboard.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Text } from 'react-native';

import QueueDetail from '@/components/QueueDetail';
import { QueueQRCard } from '@/components/QueueQRCard';
import type { TimelineItem } from '@/components/QueueTimeline';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { CounterBar } from '@/components/CounterBar';
import { useCounterSession } from '@/hooks/use-counter-session';
import { useNextSkipConfirm } from '@/hooks/use-next-skip-confirm';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useTicketNote } from '@/hooks/use-ticket-note';
import { AppendNoteModal } from '@/components/ui/AppendNoteModal';
import { useQueueQR } from '@/hooks/use-queue-qr';
import { useDisplayConfig } from '@/hooks/use-display-config';
import { DisplaySettingsCard } from '@/components/DisplaySettingsCard';
import { useSkipSettings } from '@/hooks/use-skip-settings';
import { SkipSettingsCard } from '@/components/SkipSettingsCard';
import { useQueueDetail } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { canRemoveTicket } from '@/lib/tickets';
import { closeQueue, updateQueue } from '@/lib/queues';
import { useAuthStore } from '@/store/authStore';
import type { Queue } from '@/types/api';
import { formatTicketNumber } from '@/utils/format';
import { handleMenuItemPress } from '@/utils/menu';

/**
 * Entri rute: menunggu data antrian, lalu merender editor. Editor dipasang ulang dengan key per antrian agar isian form tidak terbawa dari antrian sebelumnya.
 */
export default function QueueDetailRoute() {
  const { queueId } = useLocalSearchParams<{ queueId: string }>();
  const displayName = useAuthStore((s) => s.displayName);
  const { queue, loading, error, reload } = useQueueDetail(Number(queueId));

  if (!queue) {
    return (
      <ResponsiveScreenLayout
        title="Detail Antrian"
        userName={displayName ?? undefined}
        onMenuItemPress={handleMenuItemPress}
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
  const display = useDisplayConfig(queue.id);

  // Papan tiket: daftar tiket + SKIP/NEXT. Antrian ditutup dari sini memuat ulang detail.
  const counters = useCounterSession(queue.id);
  const board = useQueueBoard(queue.id, () => void reload(), counters.counterId);
  const currentId = board.view.current?.ticket.id ?? null;
  // Status tiket dipetakan ke timeline: yang sedang dilayani ditandai current, completed menjadi done, status lain diteruskan apa adanya.
  const timeline: TimelineItem[] = board.tickets.flatMap((t): TimelineItem[] => {
    // Tiket yang dihapus owner tidak ikut ditampilkan di timeline.
    if (t.status === 'deleted') return [];
    return [{
      number: formatTicketNumber(t.ticket_number, queue.ticket_prefix),
      status: t.id === currentId || t.status === 'currently_served' ? 'current' : t.status === 'completed' ? 'done' : t.status,
    }];
  });
  // Catatan menempel ke nomor yang sedang dilayani; tanpa nomor itu, tombol catatan nonaktif.
  // Setelah catatan disimpan, daftar tiket dimuat ulang supaya pratinjau catatan ikut berubah.
  const note = useTicketNote(queue.id, () => void board.refresh());
  const currentTicketId = board.view.current?.ticket.id ?? null;
  // Nomor yang boleh dihapus owner (4.10). Antrian yang sudah ditutup tidak menawarkan aksi ini.
  const removableTickets =
    queue.status === 'closed'
      ? []
      : board.tickets.filter(canRemoveTicket).map((t) => ({
          id: t.id,
          number: formatTicketNumber(t.ticket_number, queue.ticket_prefix),
          current: t.status === 'currently_served',
          hasNote: Boolean(t.note),
        }));
  // Semua nomor yang punya catatan owner, apa pun statusnya (catatan tetap ada walau nomor dibatalkan atau dihapus).
  const noteItems = board.tickets
    .filter((t) => t.note)
    .map((t) => ({ id: t.id, number: formatTicketNumber(t.ticket_number, queue.ticket_prefix), note: t.note as string }));
  const currentNumber = board.view.current ? formatTicketNumber(board.view.current.ticket.ticket_number, queue.ticket_prefix) : null;
  const confirm = useNextSkipConfirm({
    current: currentNumber,
    counterName: counters.counters.length > 1 ? counters.current?.name : undefined,
    onNext: board.next,
    onSkip: board.skip,
  });

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
      onMenuItemPress={handleMenuItemPress}
      queue={queue}
      queueName={queueName}
      queueDescription={queueDescription}
      onChangeQueueName={setQueueName}
      onChangeQueueDescription={setQueueDescription}
      ticketPrefix={ticketPrefix}
      onChangeTicketPrefix={setTicketPrefix}
      onSave={save}
      saving={saving}
      errorMessage={errorMessage ?? board.nextError ?? board.skipError ?? board.recallError ?? board.removeError ?? board.syncNotice ?? board.error}
      // Antrian tidak bisa diakses (board.error): tab counter ikut disembunyikan agar pesan galat tidak ganda.
      counterBar={board.error ? undefined : <CounterBar session={counters} tickets={board.tickets} prefix={queue.ticket_prefix} />}
      removableTickets={removableTickets}
      noteItems={noteItems}
      onOpenTicketNote={(ticketId) => void note.open(ticketId)}
      noteOpen={note.visible}
      onRemoveTicket={(ticketId) => void board.remove(ticketId)}
      removeLoading={board.removeLoading}
      timeline={timeline}
      currentNumber={currentNumber}
      onSkip={confirm.requestSkip}
      skipLoading={board.skipLoading}
      onNext={confirm.requestNext}
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
      displayCard={
        <DisplaySettingsCard
          config={display.config}
          loading={display.loading}
          busy={display.busy}
          error={display.error}
          onEnable={() => void display.enable()}
          onChangeCounts={(c) => void display.setCounts(c)}
          onRotate={() => void display.rotate()}
          onDisable={() => void display.disable()}
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
      onInsert={board.insert}
      insertLoading={board.insertLoading}
      insertError={board.insertError}
      defaultInsertPosition={board.tickets.filter((t) => t.status === 'waiting').length + 1}
      onCloseQueue={close}
      closing={closing}
      onOpenNote={currentTicketId !== null ? () => void note.open(currentTicketId) : undefined}
    />
    {confirm.modal}
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
