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

/** Antrian saya: tiket yang masih menunggu (customer). */
export default function AntrianSayaRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const avatarColor = useThemeColor('avatar');
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
        <EmptyState
          icon={<Icon name="list" size={44} color={avatarColor} />}
          title="Belum Ikut Antrian"
          description="Masuk ke antrian dengan tautan dari penyelenggara untuk mulai mengantri."
        />
      ) : (
        <MyTicketsList
          tickets={active}
          onOpen={(queueId) => router.push({ pathname: '/ticket/[queueId]', params: { queueId: String(queueId) } })}
        />
      )}
    </ResponsiveScreenLayout>
  );
}
