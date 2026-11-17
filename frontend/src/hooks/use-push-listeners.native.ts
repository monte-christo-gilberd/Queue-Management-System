/**
 * use-push-listeners.native.ts: listener notifikasi Android sepanjang sesi aplikasi.
 *
 * Dipakai oleh : app/_layout.tsx (sekali, di layout akar).
 * Bergantung   : expo-notifications, expo-router (router), lib/push-native.ts, lib/push-native-core.ts.
 * Alur         : startup -> handler foreground + kanal `queue-turn` -> dengarkan token FCM baru (daftar ulang) ->
 *                dengarkan ketukan notifikasi (buka data.url, mis. /ticket/12), termasuk saat cold start.
 * Catatan      : notifikasi yang datang saat aplikasi terbuka tetap ditampilkan sebagai notifikasi sistem (P2).
 */
import * as Notifications from 'expo-notifications';
import { router, type Href } from 'expo-router';
import { useEffect } from 'react';

import { nativePush } from '@/lib/push-native';
import { notificationUrl } from '@/lib/push-native-core';

/** Path tujuan dari respons ketukan. Pesan FCM mentah bisa menaruh data di content.data atau di remoteMessage. */
function urlOf(response: Notifications.NotificationResponse): string | null {
  const request = response.notification.request;
  const trigger = request.trigger as { remoteMessage?: { data?: Record<string, unknown> } } | null;
  return notificationUrl(request.content.data?.url, trigger?.remoteMessage?.data?.url);
}

/** Buka halaman tujuan notifikasi. */
function open(response: Notifications.NotificationResponse | null) {
  if (!response) return;
  const url = urlOf(response);
  if (url) router.push(url as Href);
}

// Notifikasi saat aplikasi terbuka tetap tampil (banner dan suara), sama seperti saat di background.
if (nativePush.supported()) {
  Notifications.setNotificationHandler({
    handleNotification: async () => ({
      shouldShowBanner: true,
      shouldShowList: true,
      shouldPlaySound: true,
      shouldSetBadge: false,
    }),
  });
}

/** usePushListeners: pasang listener token dan ketukan notifikasi; dilepas saat layout akar unmount. */
export function usePushListeners(): void {
  useEffect(() => {
    if (!nativePush.supported()) return;
    // Kanal dibuat sebelum notifikasi pertama datang (Android 8+ tidak menampilkan notifikasi tanpa kanal).
    void nativePush.ensureChannel();
    const tokenSub = Notifications.addPushTokenListener((t) => void nativePush.tokenChanged(String(t.data)));
    const responseSub = Notifications.addNotificationResponseReceivedListener(open);
    // Cold start: aplikasi dibuka dari ketukan notifikasi saat proses belum berjalan.
    void Notifications.getLastNotificationResponseAsync()
      .then((r) => {
        open(r);
        if (r) void Notifications.clearLastNotificationResponseAsync().catch(() => undefined);
      })
      .catch(() => undefined);
    return () => {
      tokenSub.remove();
      responseSub.remove();
    };
  }, []);
}
