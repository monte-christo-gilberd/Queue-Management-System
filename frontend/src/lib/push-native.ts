/**
 * push-native.ts: notifikasi giliran di aplikasi Android lewat FCM (expo-notifications).
 *
 * Dipakai oleh : lib/push-channel.native.ts, hooks/use-push-listeners.native.ts.
 * Bergantung   : expo-notifications (izin, kanal, token FCM), expo-constants (deteksi Expo Go), AsyncStorage,
 *                lib/api.ts (apiFetch), lib/push-native-core.ts (logika).
 * Catatan      : hanya jalan di development/production build Android. Di Expo Go dan iOS `supported()` false,
 *                sehingga tombol notifikasi tidak tampil. Web memakai lib/push.ts (Web Push), bukan file ini.
 */
import AsyncStorage from '@react-native-async-storage/async-storage';
import Constants, { ExecutionEnvironment } from 'expo-constants';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';

import { apiFetch } from '@/lib/api';
import { CHANNEL_ID, createNativePush } from '@/lib/push-native-core';
import { createSafeStorage } from '@/store/safe-storage';

/** available: FCM hanya ada di build Android sendiri, bukan di Expo Go (StoreClient). */
const available =
  Platform.OS === 'android' && Constants.executionEnvironment !== ExecutionEnvironment.StoreClient;

/** nativePush: satu instans untuk seluruh aplikasi. Setiap kemampuan di bawah membungkus satu panggilan Expo. */
export const nativePush = createNativePush({
  available,
  // Kanal `queue-turn` dengan importance HIGH: notifikasi muncul sebagai heads-up (banner di atas layar) dan bersuara.
  // Android menyimpan pengaturan kanal setelah dibuat; perubahan nama/importance di sini tidak menimpa pilihan pengguna.
  ensureChannel: async () => {
    await Notifications.setNotificationChannelAsync(CHANNEL_ID, {
      name: 'Giliran antrian',
      description: 'Pemberitahuan saat nomor Anda dipanggil atau hampir dipanggil',
      importance: Notifications.AndroidImportance.HIGH,
      vibrationPattern: [0, 250, 250, 250],
      // Warna lampu notifikasi (perangkat yang punya LED) = biru logo.
      lightColor: '#42BFF5',
    });
  },
  // Status izin POST_NOTIFICATIONS tanpa menampilkan dialog (Android 12 ke bawah selalu granted).
  permissionGranted: async () => (await Notifications.getPermissionsAsync()).granted,
  // Dialog izin sistem (Android 13+). Bila pengguna pernah memilih "Jangan izinkan" dua kali, Android tidak lagi
  // menampilkan dialog dan hasilnya langsung false.
  requestPermission: async () => (await Notifications.requestPermissionsAsync()).granted,
  // Token FCM mentah (bukan ExponentPushToken). Butuh google-services.json saat build; tanpa itu panggilan ini gagal.
  deviceToken: async () => {
    const token = await Notifications.getDevicePushTokenAsync();
    return String(token.data);
  },
  // Request ke backend memakai token login yang sama dengan seluruh aplikasi; 204 dibaca sebagai sukses.
  api: (path, init) => apiFetch(path, init),
  // AsyncStorage dibungkus supaya galat storage tidak pernah melempar (dibaca sebagai "belum aktif").
  storage: createSafeStorage(AsyncStorage),
});
