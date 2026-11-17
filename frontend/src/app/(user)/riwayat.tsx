/**
 * riwayat: riwayat saya: satu baris per tiket customer (selesai, dilewati, dibatalkan, dihapus, atau antrian ditutup).
 *
 * Rute        : /riwayat.
 * Akses       : butuh login (AuthGate dari (user)/_layout.tsx).
 * Dipakai oleh: menu panel (case riwayat-saya di utils/menu) dan tab riwayat pada ResponsiveScreenLayout.
 * Bergantung  : useMyHistory (lib/history), MyHistoryList, EmptyState.
 * Catatan     : Bila belum ada riwayat, EmptyState ditampilkan setelah pemuatan selesai.
 */
import { Text } from 'react-native';

import { MyHistoryList } from '@/components/MyHistoryList';
import ResponsiveScreenLayout from '@/components/layout/ResponsiveScreenLayout';
import { EmptyState } from '@/components/ui/EmptyState';
import { Icon } from '@/components/ui/Icon';
import { useErrorToast } from '@/components/ui/Toast';
import { useMyHistory } from '@/hooks/use-history';
import { useThemeColor } from '@/hooks/use-theme-mode';
import { useAuthStore } from '@/store/authStore';
import { handleMenuItemPress } from '@/utils/menu';

/** Riwayat tiket saya: satu baris per tiket (selesai, dilewati, dibatalkan, dihapus, atau antrian ditutup). */
export default function RiwayatSayaRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const avatarColor = useThemeColor('avatar');
  const history = useMyHistory();
  useErrorToast(history.error);

  return (
    <ResponsiveScreenLayout
      title="Riwayat"
      userName={displayName ?? undefined}
      onMenuItemPress={handleMenuItemPress}
      activeTab="riwayat">
      {history.loading ? <Text className="text-[13px] text-muted">Memuat riwayat...</Text> : null}
      {!history.loading && history.items.length === 0 ? (
        <EmptyState
          icon={<Icon name="history" size={44} color={avatarColor} />}
          title="Belum Ada Riwayat"
          description="Riwayat antrian akan muncul di sini setelah antrian Anda selesai atau ditutup."
        />
      ) : null}
      <MyHistoryList
        items={history.items}
        hasMore={history.hasMore}
        loadingMore={history.loadingMore}
        onLoadMore={() => void history.loadMore()}
      />
    </ResponsiveScreenLayout>
  );
}
