import { useLocalSearchParams } from 'expo-router';

import { DisplayScreen } from '@/components/DisplayScreen';
import { useDisplayScreen } from '@/hooks/use-display-screen';

/**
 * Layar publik antrian (monitor ruang tunggu): /display/<token>. Halaman publik tanpa AuthGate dan tanpa
 * kontrol; satu-satunya kredensial adalah token acak di tautan.
 */
export default function PublicDisplayRoute() {
  const { token } = useLocalSearchParams<{ token?: string | string[] }>();
  const raw = (Array.isArray(token) ? token[0] : token) ?? '';
  // key: token lain = layar baru dengan keadaan awal bersih.
  return <PublicDisplay key={raw} token={raw} />;
}

function PublicDisplay({ token }: { token: string }) {
  const { snapshot, status, connected, highlighted } = useDisplayScreen(token);
  return <DisplayScreen snapshot={snapshot} status={status} connected={connected} highlighted={highlighted} />;
}
