import { router } from 'expo-router';

import { MyTicketsList } from '@/components/MyTicketsList';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { EmptyState } from '@/components/ui/EmptyState';
import { Icon } from '@/components/ui/Icon';
import { useThemeColor } from '@/hooks/use-theme-mode';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { useAuthStore } from '@/store/authStore';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Riwayat tiket saya: antrian yang sudah ditutup, dilewati, atau dibatalkan. */
export default function RiwayatSayaRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const avatarColor = useThemeColor('avatar');
  const { tickets } = useMyTickets();
  const openQueue = (queueId: number) =>
    router.push({ pathname: '/ticket/[queueId]', params: { queueId: String(queueId) } });

  const closedOrEnded = tickets.filter(
    (t) => t.queue_status === 'closed' || t.status === 'skipped' || t.status === 'cancelled',
  );
  // Sudah dilayani di antrian aktif dan nomornya sudah lewat: masuk riwayat.
  const served = tickets.filter((t) => t.status === 'completed' && t.queue_status === 'active' && !isOnHome(t));
  const empty = closedOrEnded.length === 0 && served.length === 0;

  return (
    <ResponsiveScreenLayout
      title="Riwayat"
      userName={displayName ?? undefined}
      onMenuItemPress={handleOwnerMenuItemPress}
      activeTab="riwayat">
      {empty ? (
        <EmptyState
          icon={<Icon name="history" size={44} color={avatarColor} />}
          title="Belum Ada Riwayat"
          description="Riwayat antrian akan muncul di sini setelah antrian Anda selesai atau ditutup."
        />
      ) : null}
      <MyTicketsList tickets={closedOrEnded} onOpen={openQueue} />
      <MyTicketsList tickets={served} onOpen={openQueue} />
    </ResponsiveScreenLayout>
  );
}
