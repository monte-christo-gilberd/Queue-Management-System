import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';

import CreateQueueModal from '@/components/CreateQueueModal';
import { JoinQueueModals } from '@/components/ui/JoinQueueModals';
import Dashboard from '@/components/Dashboard';
import { MySpotCard, NotQueuingCard } from '@/components/MySpotCard';
import { QueueTimeline, type TimelineItem } from '@/components/QueueTimeline';
import { AppendNoteModal } from '@/components/ui/AppendNoteModal';
import { useIsWide } from '@/hooks/use-is-wide';
import { useMe } from '@/hooks/use-me';
import { useQueueSpot } from '@/hooks/use-queue-spot';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { useQueueBoard } from '@/hooks/use-queue-board';
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

  async function handleCreate(name: string, description: string) {
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
  const board = useQueueBoard(queue.id, onQueueClosed);
  const [noteOpen, setNoteOpen] = useState(false);
  const { previous, current, next, upcoming } = board.view;

  const items: TimelineItem[] = [
    ...(previous ? [{ number: formatTicketNumber(previous.ticket.ticket_number), status: 'done' as const }] : []),
    ...(current ? [{ number: formatTicketNumber(current.ticket.ticket_number), status: 'current' as const }] : []),
    ...(next ? [{ number: formatTicketNumber(next.ticket.ticket_number), status: 'waiting' as const }] : []),
    ...upcoming.map((entry) => ({
      number: formatTicketNumber(entry.ticket.ticket_number),
      status: 'waiting' as const,
    })),
  ];
  const message = board.nextError ?? board.error;

  return (
    <>
      {message ? <Text className="text-[12px] font-normal text-live">{message}</Text> : null}
      <QueueTimeline
        title={queue.queue_name}
        items={items}
        onNext={() => void board.next()}
        onNote={() => setNoteOpen(true)}
        onOpen={onOpen}
        nextLoading={board.nextLoading}
      />
      <AppendNoteModal
        visible={noteOpen}
        onCancel={() => setNoteOpen(false)}
        // TODO: backend belum mendukung catatan antrian; isi catatan belum disimpan.
        onSubmit={() => setNoteOpen(false)}
      />
    </>
  );
}

/** Kartu satu antrian aktif milik customer. Status diambil dan diperbarui sendiri lewat SSE. */
function QueueSpot({
  queueId,
  queueName,
  onChanged,
}: {
  queueId: number;
  queueName: string;
  onChanged: () => void;
}) {
  const status = useQueueSpot(queueId, onChanged);
  if (!status) return null;
  return (
    <MySpotCard
      queueName={queueName}
      currentNumber={status.current_number != null ? formatTicketNumber(status.current_number) : null}
      previousNumber={status.previous_number != null ? formatTicketNumber(status.previous_number) : null}
      nextNumber={status.next_number != null ? formatTicketNumber(status.next_number) : null}
    />
  );
}
