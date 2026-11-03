import { useErrorToast } from '@/components/ui/Toast';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { View } from 'react-native';

import CreateQueueModal from '@/components/CreateQueueModal';
import { JoinQueueModals } from '@/components/ui/JoinQueueModals';
import Dashboard from '@/components/Dashboard';
import { MySpotCard, NotQueuingCard } from '@/components/MySpotCard';
import { QueueTimeline, type TimelineItem } from '@/components/QueueTimeline';
import { AppendNoteModal } from '@/components/ui/AppendNoteModal';
import { MyTicketModal } from '@/components/ui/MyTicketModal';
import type { CustomerTicketStatus } from '@/components/CustomerQueueScreen';
import { useIsWide } from '@/hooks/use-is-wide';
import { useMe } from '@/hooks/use-me';
import { useQueueSpot } from '@/hooks/use-queue-spot';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { CounterBar } from '@/components/CounterBar';
import { useCounterSession } from '@/hooks/use-counter-session';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useTicketNote } from '@/hooks/use-ticket-note';
import { useQueueList } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { createQueue } from '@/lib/queues';
import type { Queue } from '@/types/api';
import { useAuthStore } from '@/store/authStore';
import { formatTicketNumber } from '@/utils/format';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Jumlah antrian aktif yang tampil di Dashboard owner (yang terbaru). Sisanya ada di Daftar Antrian. */
const DASHBOARD_QUEUE_COUNT = 2;

export default function DashboardRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const { queues, error: queuesError, reload: reloadQueues } = useQueueList();
  // Bagian owner (antrian kosong + buat antrian) hanya untuk owner; sebelum profil dimuat, disembunyikan.
  const { isOwner, loaded: meLoaded } = useMe();
  const [createOpen, setCreateOpen] = useState(false);
  // Dari menu panel owner ("Buat Antrian Baru") di halaman lain: parameter create=1 membuka popup.
  const { create } = useLocalSearchParams<{ create?: string }>();
  const createFromMenu = create === '1';
  const createVisible = createOpen || createFromMenu;
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();
  function closeCreate() {
    setCreateOpen(false);
    setCreateError(undefined);
    if (createFromMenu) router.setParams({ create: undefined });
  }
  const [joinOpen, setJoinOpen] = useState(false);
  const isWide = useIsWide();
  // Tiket saya (customer). Hanya tiket waiting di antrian yang masih aktif yang tampil di beranda;
  // antrian yang ditutup pindah ke Riwayat.
  const { tickets: myTickets, reload: reloadTickets } = useMyTickets();
  // Kartu hanya untuk tiket yang relevan di beranda (lihat isOnHome).
  const activeSpots = myTickets.filter(isOnHome);
  const topCard =
    meLoaded && !isOwner ? (
      activeSpots.length > 0 ? (
        <View className="w-full gap-4">
          {activeSpots.map((t) => (
            <QueueSpot
              key={t.id}
              queueId={t.queue_id}
              queueName={t.queue_name}
              ticketPrefix={t.ticket_prefix}
              onChanged={() => void reloadTickets()}
            />
          ))}
        </View>
      ) : (
        <NotQueuingCard onJoin={isWide ? () => setJoinOpen(true) : undefined} />
      )
    ) : null;

  // Dua antrian aktif terbaru (created_at paling baru). Antrian aktif lainnya ada di Daftar Antrian.
  const activeQueues = queues
    .filter((q) => q.status === 'active')
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
    .slice(0, DASHBOARD_QUEUE_COUNT);

  async function handleCreate(name: string, description: string, ticketPrefix: string) {
    const trimmed = name.trim();
    if (!trimmed) {
      setCreateError('Nama antrian wajib diisi');
      return;
    }
    setCreating(true);
    setCreateError(undefined);
    try {
      const queue = await createQueue({
        queue_name: trimmed,
        queue_description: description.trim(),
        ticket_prefix: ticketPrefix || undefined,
      });
      closeCreate();
      router.push({ pathname: '/queues/[queueId]', params: { queueId: String(queue.id) } });
    } catch (e) {
      setCreateError(describeQueueError(e));
    } finally {
      setCreating(false);
    }
  }

  const boards = activeQueues.map((queue) => (
    <OwnerQueueBoard
      key={queue.id}
      queue={queue}
      onOpen={() => router.push({ pathname: '/queues/[queueId]', params: { queueId: String(queue.id) } })}
      // Antrian ditutup (SSE queue_closed): daftar diambil ulang, kartunya hilang dari Dashboard.
      onQueueClosed={() => void reloadQueues()}
    />
  ));

  return (
    <>
      <Dashboard
        shell={{ userName: displayName ?? undefined }}
        onMenuItemPress={handleOwnerMenuItemPress}
        boards={boards}
        hasActiveQueue={activeQueues.length > 0}
        errorMessage={queuesError}
        canCreateQueue={meLoaded && isOwner}
        topCard={topCard}
      />
      <JoinQueueModals visible={joinOpen} onClose={() => setJoinOpen(false)} />

      <CreateQueueModal
        visible={createVisible}
        onClose={closeCreate}
        onSubmit={handleCreate}
        loading={creating}
        errorMessage={createError}
      />
    </>
  );
}

/** Papan satu antrian aktif milik owner. Papan dan SKIP/NEXT diurus sendiri lewat SSE. */
function OwnerQueueBoard({
  queue,
  onOpen,
  onQueueClosed,
}: {
  queue: Queue;
  onOpen: () => void;
  onQueueClosed: () => void;
}) {
  const counters = useCounterSession(queue.id);
  const board = useQueueBoard(queue.id, onQueueClosed, counters.counterId);
  // Catatan menempel ke nomor yang sedang dilayani; tanpa nomor itu, tombol catatan nonaktif.
  const note = useTicketNote(queue.id);
  const currentTicketId = board.view.current?.ticket.id ?? null;
  const { previous, current, skipped, next, upcoming } = board.view;

  const items: TimelineItem[] = [
    ...(previous ? [{ number: formatTicketNumber(previous.ticket.ticket_number, queue.ticket_prefix), status: 'done' as const }] : []),
    ...(current ? [{ number: formatTicketNumber(current.ticket.ticket_number, queue.ticket_prefix), status: 'current' as const }] : []),
    ...skipped.map((entry) => ({
      number: formatTicketNumber(entry.ticket.ticket_number, queue.ticket_prefix),
      status: 'skipped' as const,
    })),
    ...(next ? [{ number: formatTicketNumber(next.ticket.ticket_number, queue.ticket_prefix), status: 'waiting' as const }] : []),
    ...upcoming.map((entry) => ({
      number: formatTicketNumber(entry.ticket.ticket_number, queue.ticket_prefix),
      status: 'waiting' as const,
    })),
  ];
  const message = board.nextError ?? board.skipError ?? board.syncNotice ?? board.error;
  useErrorToast(message);

  return (
    <>
      {board.error ? null : <CounterBar session={counters} tickets={board.tickets} prefix={queue.ticket_prefix} />}
      <QueueTimeline
        title={queue.queue_name}
        items={items}
        onSkip={() => void board.skip()}
        onNext={() => void board.next()}
        onNote={currentTicketId !== null ? () => void note.open(currentTicketId) : undefined}
        onOpen={onOpen}
        nextLoading={board.nextLoading}
        skipLoading={board.skipLoading}
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

/** Kartu satu antrian aktif milik customer. Status diambil dan diperbarui sendiri lewat SSE. */
function QueueSpot({
  queueId,
  queueName,
  ticketPrefix,
  onChanged,
}: {
  queueId: number;
  queueName: string;
  ticketPrefix?: string;
  onChanged: () => void;
}) {
  const status = useQueueSpot(queueId, onChanged);
  const [ticketOpen, setTicketOpen] = useState(false);
  if (!status) return null;
  const format = (n: number | null | undefined) => (n != null ? formatTicketNumber(n, ticketPrefix) : null);
  const ticket = status.my_ticket;
  // Counter tempat nomor saya dilayani: cocokkan nomor di daftar serving (multi-counter).
  const myCounter = ticket ? status.serving?.find((s) => s.ticket_number === ticket.ticket_number) : undefined;
  return (
    <>
      <MySpotCard
        queueName={queueName}
        currentNumber={format(status.current_number)}
        previousNumber={format(status.previous_number)}
        nextNumber={format(status.next_number)}
        onPress={() => setTicketOpen(true)}
        hint="ketuk untuk melihat tiket Anda ▾"
      />
      <MyTicketModal
        visible={ticketOpen}
        onClose={() => setTicketOpen(false)}
        queueName={queueName}
        ticketNumber={ticket ? format(ticket.ticket_number) : null}
        ticketStatus={(ticket?.status as CustomerTicketStatus | undefined) ?? null}
        currentNumber={format(status.current_number)}
        counterName={myCounter?.counter_name ?? null}
        waitingAhead={status.waiting_ahead}
      />
    </>
  );
}
