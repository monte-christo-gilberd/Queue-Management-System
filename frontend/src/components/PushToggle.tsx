import { useErrorToast } from '@/components/ui/Toast';
import { useEffect, useState } from 'react';
import { Platform, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { disablePush, enablePush, pushSupported, syncSubscription } from '@/lib/push';

function isIOSDevice(): boolean {
  if (typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  // iPadOS 13+ melaporkan diri sebagai Mac, bedakan lewat layar sentuh.
  return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
}

/** Tombol notifikasi giliran (Web Push). Tidak tampil bila perangkat tidak mendukung (mis. Expo Go). */
export function PushToggle() {
  const [on, setOn] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string>();
  useErrorToast(message);
  const supported = pushSupported();

  useEffect(() => {
    if (!supported) return;
    let active = true;
    void syncSubscription()
      .then((v) => {
        if (active) setOn(v);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [supported]);

  if (!supported) {
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
        setMessage('Izin notifikasi ditolak di browser');
      }
    } catch {
      setMessage('Gagal mengubah notifikasi');
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
