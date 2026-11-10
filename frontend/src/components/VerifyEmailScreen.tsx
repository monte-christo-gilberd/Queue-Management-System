/**
 * VerifyEmailScreen: layar hasil verifikasi email dari tautan di surel.
 *
 * Dipakai oleh : app/verify-email.tsx
 * Bergantung   : components/auth/* (AuthLayout, AuthButtons), constants/app-theme (AppColors), expo-router
 * Peran        : publik (perilaku tombol bergantung ada-tidaknya sesi login)
 * Catatan      : pemeriksaan tautan dilakukan oleh route; komponen hanya menampilkan hasilnya.
 */
import { router } from 'expo-router';
import { ActivityIndicator, Text, View } from 'react-native';

import { AuthLayout } from '@/components/auth/AuthLayout';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { AppColors } from '@/constants/app-theme';

export type VerifyEmailScreenProps = {
  /**
   * loading: tautan sedang diperiksa; done: email terverifikasi; invalid: tautan tidak bisa dipakai;
   * error: gagal karena jaringan atau server (tautan masih bisa dicoba lagi).
   */
  mode: 'loading' | 'done' | 'invalid' | 'error';
  /** Pesan galat untuk mode `error`; jika kosong, ditampilkan teks umum "Terjadi kesalahan, coba lagi". */
  errorMessage?: string;
  /** Dipanggil saat tombol "Coba Lagi" ditekan di mode `error`. Opsional; tanpa nilai, tombol mengarah seperti tombol lanjut. */
  onRetry?: () => void;
  /** Ada sesi login di perangkat ini: tombol mengarah ke Profil/dashboard, bukan ke halaman login. */
  loggedIn: boolean;
};

/** Menampilkan status verifikasi (memeriksa, berhasil, tautan tidak berlaku, atau galat jaringan). */
export default function VerifyEmailScreen({ mode, errorMessage, onRetry, loggedIn }: VerifyEmailScreenProps) {
  // Tujuan tombol lanjut bergantung status login: sudah login ke Profil, belum ke halaman login.
  const goNext = () => router.replace(loggedIn ? '/profile' : '/');

  if (mode === 'loading') {
    return (
      <AuthLayout title="Memverifikasi Email" subtitle="Mohon tunggu sebentar">
        <View className="items-center py-4" accessibilityLabel="Memeriksa tautan">
          <ActivityIndicator color={AppColors.white} />
        </View>
      </AuthLayout>
    );
  }

  if (mode === 'done') {
    return (
      <AuthLayout title="Email Terverifikasi" subtitle="Akun Anda siap dipakai">
        <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
          Email berhasil diverifikasi. Sekarang Anda bisa bergabung ke antrian dan membuat antrian.
        </Text>
        <View className="mt-2.5">
          <PrimaryAuthButton label={loggedIn ? 'Ke Profil' : 'Ke Halaman Login'} onPress={goNext} loading={false} />
        </View>
      </AuthLayout>
    );
  }

  if (mode === 'error') {
    return (
      <AuthLayout title="Verifikasi Gagal" subtitle="Tautan belum bisa diperiksa">
        <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
          {errorMessage ?? 'Terjadi kesalahan, coba lagi'}
        </Text>
        <View className="mt-2.5">
          <PrimaryAuthButton label="Coba Lagi" onPress={onRetry ?? goNext} loading={false} />
        </View>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Tautan Tidak Berlaku" subtitle="Tautan salah, kedaluwarsa, atau sudah dipakai">
      <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
        {loggedIn
          ? 'Buka halaman Profil lalu minta tautan verifikasi yang baru.'
          : 'Masuk ke akun Anda, lalu minta tautan verifikasi yang baru di halaman Profil.'}
      </Text>
      <View className="mt-2.5">
        <PrimaryAuthButton label={loggedIn ? 'Ke Profil' : 'Ke Halaman Login'} onPress={goNext} loading={false} />
      </View>
    </AuthLayout>
  );
}
