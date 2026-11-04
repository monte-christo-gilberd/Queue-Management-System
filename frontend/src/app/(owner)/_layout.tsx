import { Stack } from 'expo-router';
import { useEffect } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import { OwnerContext } from '@/components/layout/owner-context';
import { useMe } from '@/hooks/use-me';
import { useAvatarStore } from '@/store/avatarStore';

// Semua route di grup ini butuh login. Menu khusus owner dibedakan lewat OwnerContext.
export default function OwnerLayout() {
  return (
    <AuthGate mode="required">
      <OwnerProvider>
        <Stack screenOptions={{ headerShown: false }} />
      </OwnerProvider>
    </AuthGate>
  );
}

function OwnerProvider({ children }: { children: React.ReactNode }) {
  const { me, isOwner } = useMe();
  const setAvatarUpdatedAt = useAvatarStore((s) => s.setUpdatedAt);
  // Header di semua layar membaca penanda foto dari store; di sini diisi dari profil yang baru dimuat.
  useEffect(() => {
    if (me) setAvatarUpdatedAt(me.avatar_updated_at);
  }, [me, setAvatarUpdatedAt]);
  return <OwnerContext.Provider value={isOwner}>{children}</OwnerContext.Provider>;
}
