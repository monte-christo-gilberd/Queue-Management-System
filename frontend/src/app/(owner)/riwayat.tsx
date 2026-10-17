import { router } from 'expo-router';
import { Text } from 'react-native';

import { MyTicketsList } from '@/components/MyTicketsList';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { useAuthStore } from '@/store/authStore';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Riwayat tiket saya: antrian yang sudah ditutup, dilewati, atau dibatalkan. */
export default function RiwayatSayaRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const { tickets } = useMyTickets();
  const openQueue = (queueId: number) =>
    router.push({ pathname: '/join/[queueId]', params: { queueId: String(queueId) } });

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
      {empty ? <Text className="text-[13px] text-muted">Belum ada riwayat antrian.</Text> : null}
      <MyTicketsList tickets={closedOrEnded} onOpen={openQueue} />
      <MyTicketsList tickets={served} onOpen={openQueue} />
    </ResponsiveScreenLayout>
  );
}
