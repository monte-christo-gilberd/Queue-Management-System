import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { View } from 'react-native';

import CreateQueueModal from '@/components/CreateQueueModal';
import { JoinQueueModals } from '@/components/ui/JoinQueueModals';
import Dashboard from '@/components/Dashboard';
import { MySpotCard, NotQueuingCard } from '@/components/MySpotCard';
import { useIsWide } from '@/hooks/use-is-wide';
import { useMe } from '@/hooks/use-me';
import { useQueueSpot } from '@/hooks/use-queue-spot';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { useQueueBoard } from '@/hooks/use-queue-board';
import { useQueueList } from '@/hooks/use-queues';
import { describeQueueError } from '@/lib/queue-errors';
import { createQueue } from '@/lib/queues';
import { useAuthStore } from '@/store/authStore';
import { formatTicketNumber } from '@/utils/format';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

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
  const activeQueue = queues.find((q) => q.status === 'active') ?? null;
  // Antrian aktif pertama. Saat ditutup (SSE queue_closed), daftar diambil ulang dan papan kosong.
  const board = useQueueBoard(activeQueue?.id ?? null, () => {
    void reloadQueues();
  });
  const { previous, current, next, upcoming } = board.view;

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();

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

  return (
    <>
      <Dashboard
        shell={{ userName: displayName ?? undefined }}
        onMenuItemPress={handleOwnerMenuItemPress}
        activeQueue={activeQueue}
        previous={previous}
        current={current}
        next={next}
        upcoming={upcoming}
        nextLoading={board.nextLoading}
        errorMessage={board.nextError ?? board.error ?? queuesError}
        onNext={() => void board.next()}
        onOpenQueue={(queueId) => router.push({ pathname: '/queues/[queueId]', params: { queueId: String(queueId) } })}
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
