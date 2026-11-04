import * as WebBrowser from 'expo-web-browser';
import { useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Platform } from 'react-native';

import Subscription from '@/components/Subscription';
import { describePaymentError, startCheckout } from '@/lib/payments';
import { getMe, type Me } from '@/lib/users';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';
import { subscriptionStatus } from '@/utils/profile';

// Setelah kembali dari Stripe, webhook bisa tiba beberapa detik setelah redirect browser: profil dimuat ulang
// beberapa kali. Sumber kebenaran tetap webhook di payment-service, bukan redirect ini.
const POLL_ATTEMPTS = 6;
const POLL_INTERVAL_MS = 2000;

const MSG_SUCCESS = 'Pembayaran diterima. Status langganan diperbarui otomatis; bila tanggal belum berubah, tunggu sebentar lalu tekan Refresh.';
const MSG_CANCEL = 'Pembayaran dibatalkan. Anda bisa mencobanya lagi kapan saja.';

export default function SubscriptionRoute() {
  // Web: Stripe mengembalikan browser ke /subscription?payment=success|cancel.
  const { payment } = useLocalSearchParams<{ payment?: string }>();
  const [me, setMe] = useState<Me | null>(null);
  const [paying, setPaying] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [nativeMessage, setNativeMessage] = useState('');
  const [errorMessage, setErrorMessage] = useState('');
  const alive = useRef(true);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  // Muat profil; bila baru kembali dari pembayaran, ulangi beberapa kali sampai webhook tercatat.
  useEffect(() => {
    let active = true;
    const load = () =>
      getMe().then(
        (data) => {
          if (active) setMe(data);
        },
        () => {
          // Gagal memuat profil tidak boleh mengganggu layar ini; tombol REFRESH tersedia.
        },
      );
    void load();
    if (payment !== 'success') {
      return () => {
        active = false;
      };
    }
    let tries = 0;
    const timer = setInterval(() => {
      tries += 1;
      void load();
      if (tries >= POLL_ATTEMPTS) clearInterval(timer);
    }, POLL_INTERVAL_MS);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [payment]);

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
    setNativeMessage('');
    setPaying(true);
    try {
      const { checkout_url } = await startCheckout();
      if (Platform.OS === 'web') {
        window.location.assign(checkout_url);
        return;
      }
      await WebBrowser.openBrowserAsync(checkout_url);
      // Native: tidak ada redirect ke halaman ini, jadi muat ulang beberapa kali setelah browser ditutup.
      setNativeMessage('Memeriksa status pembayaran…');
      for (let i = 0; i < POLL_ATTEMPTS && alive.current; i++) {
        await reload();
        await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
      }
      if (alive.current) setNativeMessage('Jika tanggal langganan belum berubah, tunggu sebentar lalu tekan Refresh.');
    } catch (e) {
      setErrorMessage(describePaymentError(e));
    } finally {
      if (alive.current) setPaying(false);
    }
  }

  async function onRefresh() {
    setRefreshing(true);
    await reload();
    if (alive.current) setRefreshing(false);
  }

  const returnMessage = payment === 'success' ? MSG_SUCCESS : payment === 'cancel' ? MSG_CANCEL : '';
  const status = me?.is_owner ? subscriptionStatus(me.subscription_expires_at) : null;
  return (
    <Subscription
      paid={status?.active ?? false}
      title={status ? (status.active ? 'Langganan Aktif' : 'Langganan Belum Aktif') : ''}
      subtitle={status?.description ?? ''}
      username={me?.username ?? ''}
      onPay={pay}
      paying={paying}
      message={nativeMessage || returnMessage}
      errorMessage={errorMessage}
      onRefresh={onRefresh}
      refreshing={refreshing}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
