import { router } from 'expo-router';

import { MyTicketsList } from '@/components/MyTicketsList';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { Text } from 'react-native';
import { useMyTickets } from '@/hooks/use-my-tickets';
import { isOnHome } from '@/lib/tickets';
import { useAuthStore } from '@/store/authStore';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

/** Antrian saya: tiket yang masih menunggu (customer). */
export default function AntrianSayaRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const { tickets } = useMyTickets();
  // Sama dengan beranda: menunggu di antrian aktif, atau nomornya sedang dilayani.
  const active = tickets.filter(isOnHome);

  return (
    <ResponsiveScreenLayout
      title="Antrian Saya"
      userName={displayName ?? undefined}
      onMenuItemPress={handleOwnerMenuItemPress}
      activeTab="antrian">
      {active.length === 0 ? (
        <Text className="text-[13px] text-muted">Anda belum mengikuti antrian apa pun.</Text>
      ) : (
        <MyTicketsList
          tickets={active}
          onOpen={(queueId) => router.push({ pathname: '/join/[queueId]', params: { queueId: String(queueId) } })}
        />
      )}
    </ResponsiveScreenLayout>
  );
}
