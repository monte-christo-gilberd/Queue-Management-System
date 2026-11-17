/**
 * PushToggle: tombol untuk mengaktifkan atau mematikan notifikasi giliran.
 *
 * Dipakai oleh : components/CustomerQueueScreen.tsx
 * Bergantung   : lib/push-channel (pushSupported, syncPush, enablePush, disablePush, deniedMessage), ui/AppButton, ui/Toast
 * Peran        : customer
 * Catatan      : web memakai Web Push (lib/push.ts), aplikasi Android memakai FCM (lib/push-native.ts); Metro memilih
 *                push-channel.native.ts di native. Di Expo Go dan iOS komponen tidak tampil (FCM tidak tersedia).
 */
import { useErrorToast } from '@/components/ui/Toast';
import { useEffect, useState } from 'react';
import { Platform, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { deniedMessage, disablePush, enablePush, pushSupported, syncPush } from '@/lib/push-channel';

function isIOSDevice(): boolean {
  if (typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  // iPadOS 13+ melaporkan diri sebagai Mac, bedakan lewat layar sentuh.
  return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
}

/** Tombol notifikasi giliran (Web Push atau FCM). Tidak tampil bila perangkat tidak mendukung (mis. Expo Go). */
export function PushToggle() {
  const [on, setOn] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string>();
  useErrorToast(message);
  const supported = pushSupported();

  useEffect(() => {
    if (!supported) return;
    // Flag `active` mencegah setState dijalankan setelah komponen unmount.
    let active = true;
    // Status langganan yang tersimpan dibaca saat mount agar tombol sesuai kondisi sebenarnya.
    void syncPush()
      .then((v) => {
        if (active) setOn(v);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [supported]);

  if (!supported) {
    // Di native tidak ada pesan bantuan; hanya web yang diberi petunjuk cara mengaktifkan notifikasi.
    if (Platform.OS !== 'web') return null;
    return (
      <Text className="text-[12px] font-normal text-muted">
        {isIOSDevice()
          ? 'Di iPhone/iPad, notifikasi hanya bisa setelah situs dipasang: buka di Safari, ketuk Bagikan, pilih "Tambahkan ke Layar Utama", lalu buka dari ikon itu.'
          : 'Notifikasi tidak didukung di browser ini. Gunakan Chrome, Edge, atau Firefox.'}
      </Text>
    );
  }

  const toggle = async () => {
    setBusy(true);
    setMessage(undefined);
    try {
      if (on) {
        await disablePush();
        setOn(false);
      } else if (await enablePush()) {
        setOn(true);
      } else {
        // enablePush mengembalikan false bila pengguna menolak izin notifikasi (browser atau Android).
        setMessage(deniedMessage);
      }
    } catch (err) {
      // Alasan asli disertakan agar penyebab (mis. layanan push diblokir browser) terlihat, bukan hanya pesan umum.
      console.warn('[push] gagal mengubah notifikasi:', err);
      const reason = err instanceof Error && err.message ? `: ${err.message}` : '';
      setMessage(`Gagal mengubah notifikasi${reason}`);
    }
    setBusy(false);
  };

  return (
    <View className="gap-2">
      <AppButton
        label={on ? 'MATIKAN NOTIFIKASI' : 'AKTIFKAN NOTIFIKASI'}
        variant="outline"
        onPress={() => void toggle()}
        loading={busy}
      />
    </View>
  );
}
