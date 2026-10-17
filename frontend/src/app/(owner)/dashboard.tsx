import { router } from 'expo-router';
import { useState } from 'react';

import CreateQueueModal from '@/components/CreateQueueModal';
import Dashboard from '@/components/Dashboard';
import { MyTicketsList } from '@/components/MyTicketsList';
import { useMe } from '@/hooks/use-me';
import { useMyTickets } from '@/hooks/use-my-tickets';
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
  // Tiket saya (customer yang sudah bergabung) ditampilkan di bawah papan.
  const { tickets: myTickets } = useMyTickets();
  const activeQueue = queues.find((q) => q.status === 'active') ?? null;
  // Antrian aktif pertama. Saat ditutup (SSE queue_closed), daftar diambil ulang dan papan kosong.
  const board = useQueueBoard(activeQueue?.id ?? null, () => {
    void reloadQueues();
  });
  const { previous, current, next, upcoming } = board.view;

  const [createOpen, setCreateOpen] = useState(false);
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
      setCreateOpen(false);
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
        notificationMessage={
          current ? `Nomor ${formatTicketNumber(current.ticket.ticket_number)} sedang dilayani` : undefined
        }
        nextLoading={board.nextLoading}
        errorMessage={board.nextError ?? board.error ?? queuesError}
        onNext={() => void board.next()}
        onCreateQueue={() => setCreateOpen(true)}
        canCreateQueue={meLoaded && isOwner}
        footer={
          <MyTicketsList
            tickets={myTickets}
            onOpen={(queueId) => router.push({ pathname: '/join/[queueId]', params: { queueId: String(queueId) } })}
          />
        }
      />
      <CreateQueueModal
        visible={createOpen}
        onClose={() => {
          setCreateOpen(false);
          setCreateError(undefined);
        }}
        onSubmit={handleCreate}
        loading={creating}
        errorMessage={createError}
      />
    </>
  );
}
