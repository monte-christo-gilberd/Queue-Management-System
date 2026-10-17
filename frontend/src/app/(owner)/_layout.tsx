import { Stack } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';
import { OwnerContext } from '@/components/layout/owner-context';
import { useMe } from '@/hooks/use-me';

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
  const { isOwner } = useMe();
  return <OwnerContext.Provider value={isOwner}>{children}</OwnerContext.Provider>;
}
