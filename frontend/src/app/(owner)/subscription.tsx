/**
 * subscription: langganan owner: status aktif, tombol bayar satu bulan, dan tombol jadi owner untuk calon owner.
 *
 * Rute        : /subscription.
 * Akses       : butuh login (AuthGate dari (owner)/_layout.tsx).
 * Dipakai oleh: Profil (tombol langganan atau Jadi Owner).
 * Bergantung  : startSubscriptionCheckout dan describeSubscriptionError (lib/subscription), getMe (lib/users), Subscription.
 * Catatan     : Tombol membuat pesanan lalu membuka halaman BorderPay. Langganan diperbarui oleh webhook setelah lunas.
 */
import { useEffect, useRef, useState } from 'react';
import { Linking, Platform } from 'react-native';

import Subscription from '@/components/Subscription';
import { describeSubscriptionError, startSubscriptionCheckout } from '@/lib/subscription';
import { getMe, type Me } from '@/lib/users';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';
import { useAuthStore } from '@/store/authStore';
import { subscriptionStatus } from '@/utils/profile';

const MSG_DONE = 'Selesaikan pembayaran di halaman BorderPay. Langganan aktif otomatis setelah pembayaran diterima; tekan REFRESH untuk memperbarui.';

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

  async function pay() {
    setErrorMessage('');
    setDoneMessage('');
    setPaying(true);
    try {
      const checkout = await startSubscriptionCheckout();
      // Buka halaman pembayaran BorderPay. Di web lewat tab baru; di native lewat browser sistem.
      if (Platform.OS === 'web') {
        window.open(checkout.pay_url, '_blank', 'noopener');
      } else {
        await Linking.openURL(checkout.pay_url);
      }
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
  // Calon owner: pembayaran pertama yang lunas menjadikan akun ini owner.
  const prospect = me !== null && !me.is_owner;
  return (
    <Subscription
      shell={{ userName: displayName ?? undefined }}
      paid={status?.active ?? false}
      title={status ? (status.active ? 'Langganan Aktif' : 'Langganan Belum Aktif') : prospect ? 'Belum Jadi Owner' : ''}
      subtitle={status?.description ?? (prospect ? 'Bayar satu bulan untuk menjadi owner dan mulai membuat antrian.' : '')}
      payTitle={prospect ? 'Jadi Owner' : undefined}
      payHint={prospect ? 'Bayar satu bulan dengan QRIS. Akun Anda menjadi owner setelah pembayaran diterima.' : undefined}
      payLabel={prospect ? 'JADI OWNER (BAYAR 1 BULAN)' : undefined}
      username={me?.username ?? ''}
      onPay={pay}
      paying={paying}
      message={doneMessage}
      errorMessage={errorMessage}
      onRefresh={onRefresh}
      refreshing={refreshing}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
