/**
 * (owner)/_layout: layout grup (owner): penjaga login dan penyedia konteks owner untuk semua halaman di dalam folder (owner).
 *
 * Rute        : semua rute di dalam grup, misalnya /dashboard, /antrian, /riwayat, /queues, /history, /profile, /subscription. Nama grup tidak masuk ke URL.
 * Akses       : butuh login (AuthGate mode required).
 * Dipakai oleh: Expo Router untuk seluruh rute di grup ini.
 * Bergantung  : AuthGate, OwnerContext, useMe, useAvatarStore.
 * Catatan     : Nama grup "(owner)" menyesatkan: layar customer (dashboard, antrian, riwayat) juga ada di grup ini. Menu khusus owner dibedakan lewat OwnerContext.
 */
import { Stack } from 'expo-router';
import { useEffect } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import { OwnerContext } from '@/components/layout/owner-context';
import { useMe } from '@/hooks/use-me';
import { useAvatarStore } from '@/store/avatarStore';

/**
 * Semua rute di grup ini butuh login (AuthGate mode required), lalu OwnerProvider memasang konteks owner.
 * Nama grup "(owner)" tidak berarti hanya owner: dashboard, antrian, dan riwayat customer juga ada di sini.
 * Menu khusus owner dibedakan lewat OwnerContext, bukan lewat pemisahan route.
 */
export default function OwnerLayout() {
  return (
    <AuthGate mode="required">
      <OwnerProvider>
        <Stack screenOptions={{ headerShown: false }} />
      </OwnerProvider>
    </AuthGate>
  );
}

/**
 * Mengambil profil sekali untuk seluruh grup: isOwner masuk ke OwnerContext, penanda foto profil masuk ke avatarStore.
 */
function OwnerProvider({ children }: { children: React.ReactNode }) {
  const { me, isOwner } = useMe();
  const setAvatarUpdatedAt = useAvatarStore((s) => s.setUpdatedAt);
  // Header di semua layar membaca penanda foto dari store; di sini diisi dari profil yang baru dimuat.
  useEffect(() => {
    if (me) setAvatarUpdatedAt(me.avatar_updated_at);
  }, [me, setAvatarUpdatedAt]);
  return <OwnerContext.Provider value={isOwner}>{children}</OwnerContext.Provider>;
}
