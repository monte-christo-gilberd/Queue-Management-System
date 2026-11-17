/**
 * AuthGate: penjaga rute yang mengarahkan user sesuai status login.
 *
 * Dipakai oleh : app/index.tsx, app/login.tsx, app/join.tsx, app/register.tsx, app/link-google.tsx, app/(user)/_layout.tsx, app/ticket/[queueId].tsx
 * Bergantung   : hooks/use-auth-hydrated, lib/auth-gate (decideAuthGate), store/authStore
 * Alur         : hydrate selesai -> decideAuthGate -> wait (layar kosong) | redirect (Redirect) | render (children)
 * Catatan      : Hanya membaca token dari authStore. Komponen ini mengimpor store, berbeda dari aturan
 *                ZUSTAND_GUIDE untuk komponen tampilan.
 */
import { Redirect, useLocalSearchParams, usePathname, type Href } from 'expo-router';
import { View } from 'react-native';

import { useAuthHydrated } from '@/hooks/use-auth-hydrated';
import { decideAuthGate, type AuthGateMode } from '@/lib/auth-gate';
import { useAuthStore } from '@/store/authStore';

/** Props AuthGate. */
type AuthGateProps = {
  /** Konten yang dirender bila keputusan akhirnya render. */
  children: React.ReactNode;
  /**
   * `required` (default): hanya untuk yang sudah login (mis. grup (user)). Tanpa token → `/login?next=<path>`
   * (tanpa `next` bila path sudah `/`).
   * `guest`: hanya untuk yang belum login (login, register). Sudah punya token → `next` bila aman
   * (lib/safe-next), selain itu `/dashboard`.
   */
  mode?: AuthGateMode;
  /**
   * Tujuan eksplisit setelah login bila path saat ini punya query penting, mis.
   * `/join?t=<token>`. Tanpa ini `usePathname()` hanya memberi `/join` dan token hilang.
   */
  nextPath?: string;
};

/**
 * Penjaga rute. Hanya membaca status; keputusannya ada di `decideAuthGate` (lib/auth-gate.ts).
 * Menunggu hydrate supaya token yang sebenarnya sudah ada tidak dianggap belum login.
 */
export function AuthGate({ children, mode = 'required', nextPath }: AuthGateProps) {
  // Status hydrate: false sampai penyimpanan lokal selesai dibaca.
  const hydrated = useAuthHydrated();
  // Token login saat ini; kosong berarti belum masuk.
  const token = useAuthStore((s) => s.token);
  const routerPath = usePathname();
  // Build statis: usePathname() bisa "/" saat hidrasi awal, sehingga `?next=` hilang.
  // Di web, path sebenarnya ada di window.location.
  const path = typeof window !== 'undefined' && window.location?.pathname ? window.location.pathname : routerPath;
  // Parameter `next` dari URL; hanya dipakai keputusan mode guest.
  const { next } = useLocalSearchParams<{ next?: string }>();

  // Keputusan murni dari lib/auth-gate; komponen ini hanya menerjemahkannya ke tampilan.
  const decision = decideAuthGate({ mode, hydrated, token, path, nextPath, next });

  // Saat 'wait' tampil layar kosong bg-background agar konten terlarang tidak sempat terlihat sekilas.
  if (decision.kind === 'wait') return <View className="flex-1 bg-background" />;
  // Redirect mengganti rute (bukan push) supaya tombol kembali tidak memantul ke halaman terlarang.
  if (decision.kind === 'redirect') return <Redirect href={decision.to as Href} />;
  // Boleh lanjut: tampilkan konten asli.
  return <>{children}</>;
}
