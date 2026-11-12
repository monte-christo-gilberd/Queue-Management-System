/**
 * subscription: langganan owner (MOCK): status aktif, tombol tambah 30 hari, dan tombol jadi owner untuk calon owner.
 *
 * Rute        : /subscription.
 * Akses       : butuh login (AuthGate dari (owner)/_layout.tsx).
 * Dipakai oleh: Profil (tombol langganan atau Jadi Owner).
 * Bergantung  : extendSubscription dan describeSubscriptionError (lib/subscription), getMe (lib/users), Subscription.
 * Catatan     : Tidak ada pembayaran. Tombol langsung menambah 30 hari lewat backend.
 */
import { useEffect, useRef, useState } from 'react';

import Subscription from '@/components/Subscription';
import { describeSubscriptionError, extendSubscription } from '@/lib/subscription';
import { refreshMe } from '@/hooks/use-me';
import { getMe, type Me } from '@/lib/users';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';
import { useAuthStore } from '@/store/authStore';
import { subscriptionStatus } from '@/utils/profile';

const MSG_DONE = 'Masa langganan bertambah 30 hari.';

/** Layar langganan. Status yang tampil selalu berasal dari profil (getMe). */
export default function SubscriptionRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const [me, setMe] = useState<Me | null>(null);
  const [paying, setPaying] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [doneMessage, setDoneMessage] = useState('');
  const [errorMessage, setErrorMessage] = useState('');
  const alive = useRef(true);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  // Muat profil saat layar dibuka. Gagal memuat tidak mengganggu layar; tombol REFRESH tersedia.
  useEffect(() => {
    getMe().then(
      (data) => {
        if (alive.current) setMe(data);
      },
      () => {},
    );
  }, []);

  async function reload() {
    try {
      const data = await getMe();
      if (alive.current) setMe(data);
    } catch {
      // lihat catatan di atas
    }
  }

  async function extend() {
    setErrorMessage('');
    setDoneMessage('');
    setPaying(true);
    try {
      await extendSubscription();
      // Muat ulang profil agar tanggal berakhir yang tampil sudah yang baru.
      await reload();
      // Perbarui status owner di layout (menu panel kanan) tanpa refresh.
      await refreshMe();
      if (alive.current) setDoneMessage(MSG_DONE);
    } catch (e) {
      if (alive.current) setErrorMessage(describeSubscriptionError(e));
    } finally {
      if (alive.current) setPaying(false);
    }
  }

  async function onRefresh() {
    setRefreshing(true);
    await reload();
    if (alive.current) setRefreshing(false);
  }

  const status = me?.is_owner ? subscriptionStatus(me.subscription_expires_at) : null;
  // Calon owner: 30 hari pertama langsung menjadikan akun ini owner.
  const prospect = me !== null && !me.is_owner;
  return (
    <Subscription
      shell={{ userName: displayName ?? undefined }}
      paid={status?.active ?? false}
      title={status ? (status.active ? 'Langganan Aktif' : 'Langganan Belum Aktif') : prospect ? 'Belum Jadi Owner' : ''}
      subtitle={status?.description ?? (prospect ? 'Tambah 30 hari untuk menjadi owner dan mulai membuat antrian.' : '')}
      payTitle={prospect ? 'Jadi Owner' : undefined}
      payHint={prospect ? '[MOCK] Tanpa pembayaran. Akun Anda langsung menjadi owner dengan langganan aktif 30 hari.' : undefined}
      payLabel={prospect ? 'JADI OWNER (30 HARI)' : undefined}
      username={me?.username ?? ''}
      onPay={extend}
      paying={paying}
      message={doneMessage}
      errorMessage={errorMessage}
      onRefresh={onRefresh}
      refreshing={refreshing}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
