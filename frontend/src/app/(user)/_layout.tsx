/**
 * (user)/_layout: layout grup (user): penjaga login dan penyedia sesi user (profil, penanda owner, foto profil) untuk
 * semua halaman setelah login.
 *
 * Rute        : semua rute di dalam grup, misalnya /dashboard, /antrian, /riwayat, /queues, /history, /profile, /subscription. Nama grup tidak masuk ke URL.
 * Akses       : butuh login (AuthGate mode required). Owner dan customer sama-sama memakai grup ini.
 * Dipakai oleh: Expo Router untuk seluruh rute di grup ini.
 * Bergantung  : AuthGate, OwnerContext, useMe, useAvatarStore.
 * Catatan     : Menu khusus owner dibedakan lewat OwnerContext, bukan lewat pemisahan route.
 */
import { Stack } from 'expo-router';
import { useEffect } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import { OwnerContext } from '@/components/layout/owner-context';
import { useMe } from '@/hooks/use-me';
import { useAvatarStore } from '@/store/avatarStore';

/**
 * Semua rute di grup ini butuh login (AuthGate mode required), lalu UserSessionProvider memasang sesi user.
 */
export default function UserLayout() {
  return (
    <AuthGate mode="required">
      <UserSessionProvider>
        <Stack screenOptions={{ headerShown: false }} />
      </UserSessionProvider>
    </AuthGate>
  );
}

/**
 * Mengambil profil sekali untuk seluruh grup: isOwner masuk ke OwnerContext, penanda foto profil masuk ke avatarStore.
 */
function UserSessionProvider({ children }: { children: React.ReactNode }) {
  const { me, isOwner } = useMe();
  const setAvatarUpdatedAt = useAvatarStore((s) => s.setUpdatedAt);
  // Header di semua layar membaca penanda foto dari store; di sini diisi dari profil yang baru dimuat.
  useEffect(() => {
    if (me) setAvatarUpdatedAt(me.avatar_updated_at);
  }, [me, setAvatarUpdatedAt]);
  return <OwnerContext.Provider value={isOwner}>{children}</OwnerContext.Provider>;
}
