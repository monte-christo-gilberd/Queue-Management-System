import { Stack } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';

// Semua route owner hanya bisa diakses setelah login. Tanpa token → halaman login.
export default function OwnerLayout() {
  return (
    <AuthGate mode="required">
      <Stack screenOptions={{ headerShown: false }} />
    </AuthGate>
  );
}
