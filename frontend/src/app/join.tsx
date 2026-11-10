/**
 * join: halaman informasi untuk tautan /join?t=<token> (QR antrian), tanpa melakukan join.
 *
 * Rute        : /join (query t diabaikan).
 * Akses       : butuh login (AuthGate mode required); tamu diarahkan ke /.
 * Dipakai oleh: tautan dari QR antrian (/join?t=<token>).
 * Bergantung  : AuthGate, AppButton.
 * Catatan     : Bergabung hanya lewat popup Masuk ke Antrian di dashboard dengan memindai QR. Token di URL tidak dibaca di sini, jadi tidak perlu nextPath.
 */
import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { AuthGate } from '@/components/auth/AuthGate';
import { AppButton } from '@/components/ui/AppButton';

/**
 * Halaman ini sengaja tidak melakukan join. Token di URL diabaikan: bergabung hanya bisa lewat
 * popup "Masuk ke Antrian" yang memindai QR dengan kamera aplikasi.
 */
export default function JoinInfoRoute() {
  return (
    <AuthGate mode="required">
      <View className="flex-1 items-center justify-center gap-4 bg-background p-6">
        <Text className="text-center text-[20px] font-extrabold text-foreground">Pindai QR di aplikasi</Text>
        <Text className="text-center text-[14px] text-muted">
          Untuk bergabung, buka Masuk ke Antrian di dashboard lalu arahkan kamera ke QR antrian.
        </Text>
        <AppButton label="Ke Dashboard" onPress={() => router.replace('/dashboard')} />
      </View>
    </AuthGate>
  );
}
