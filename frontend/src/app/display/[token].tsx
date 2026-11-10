/**
 * display/[token]: layar publik antrian (monitor ruang tunggu) untuk tautan /display/<token>.
 *
 * Rute        : /display/[token].
 * Akses       : publik, tanpa AuthGate dan tanpa kontrol; token acak di path adalah satu-satunya kredensial.
 * Dipakai oleh: tautan yang dibuat owner di kartu Pengaturan Display (detail antrian), lalu dibuka di monitor.
 * Bergantung  : useDisplayScreen (data lewat GET /display/:token), DisplayScreen.
 * Catatan     : Token salah, sudah diganti (rotate), atau layar dimatikan menghasilkan 404 dari API.
 */
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
