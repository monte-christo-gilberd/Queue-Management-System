import { useEffect, useState } from 'react';
import { Platform, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { disablePush, enablePush, pushSupported, syncSubscription } from '@/lib/push';

/** Tombol notifikasi giliran (Web Push). Tidak tampil bila perangkat tidak mendukung (mis. Expo Go). */
export function PushToggle() {
  const [on, setOn] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string>();
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
    return Platform.OS === 'web' ? (
      <Text className="text-[12px] font-normal text-muted">
        Notifikasi tidak didukung di browser ini (di iPhone, tambahkan situs ke Layar Utama dulu).
      </Text>
    ) : null;
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
      {message ? <Text className="text-[12px] font-normal text-live">{message}</Text> : null}
    </View>
  );
}
