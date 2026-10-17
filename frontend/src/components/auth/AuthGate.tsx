import { Redirect, useLocalSearchParams, usePathname, type Href } from 'expo-router';
import { View } from 'react-native';

import { useAuthHydrated } from '@/hooks/use-auth-hydrated';
import { decideAuthGate, type AuthGateMode } from '@/lib/auth-gate';
import { useAuthStore } from '@/store/authStore';

type AuthGateProps = {
  children: React.ReactNode;
  /**
   * `required` (default): hanya untuk yang sudah login (mis. area owner). Tanpa token → `/?next=<path>`.
   * `guest`: hanya untuk yang belum login (login, register). Sudah punya token → `/dashboard`.
   */
  mode?: AuthGateMode;
};

/**
 * Penjaga rute. Hanya membaca status; keputusannya ada di `decideAuthGate` (lib/auth-gate.ts).
 * Menunggu hydrate supaya token yang sebenarnya sudah ada tidak dianggap belum login.
 */
export function AuthGate({ children, mode = 'required' }: AuthGateProps) {
  const hydrated = useAuthHydrated();
  const token = useAuthStore((s) => s.token);
  const path = usePathname();
  const { next } = useLocalSearchParams<{ next?: string }>();

  const decision = decideAuthGate({ mode, hydrated, token, path, next });

  if (decision.kind === 'wait') return <View className="flex-1 bg-background" />;
  if (decision.kind === 'redirect') return <Redirect href={decision.to as Href} />;
  return <>{children}</>;
}
