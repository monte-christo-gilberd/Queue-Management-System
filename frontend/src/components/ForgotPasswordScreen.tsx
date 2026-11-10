/**
 * ForgotPasswordScreen: layar meminta tautan reset password lewat email.
 *
 * Dipakai oleh : app/forgot-password.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik
 * Catatan      : subtitle "Tautan berlaku 30 menit" harus sesuai masa berlaku tautan di backend.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { AtIcon } from '@/components/ui/Icon';

/** Props layar lupa password. */
export type ForgotPasswordScreenProps = {
  /** Email yang diisi pengguna; dikirim ke route untuk diminta tautan reset. */
  email: string;
  /** Dipanggil setiap email berubah, dengan nilai terbaru. */
  onChangeEmail: (value: string) => void;
  /** Dipanggil saat tombol Kirim ditekan atau tombol Enter pada field email. */
  onSubmit: () => void;
  /** true selama permintaan berjalan; tombol Kirim menampilkan status loading. */
  loading: boolean;
  /** Pesan galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
  /** Terisi setelah permintaan diterima: form diganti pesan konfirmasi (netral, tidak menyebut email terdaftar). */
  sentMessage?: string;
};

/** Form email untuk reset password, atau pesan konfirmasi setelah permintaan diterima. */
export default function ForgotPasswordScreen({
  email,
  onChangeEmail,
  onSubmit,
  loading,
  errorMessage,
  sentMessage,
}: ForgotPasswordScreenProps) {
  useErrorToast(errorMessage);

  // Pesan konfirmasi dibuat netral agar tidak memberi tahu apakah email tersebut terdaftar.
  if (sentMessage) {
    return (
      <AuthLayout title="Cek Email Anda" subtitle="Tautan berlaku 30 menit">
        <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
          {sentMessage}
        </Text>
        <View className="mt-2.5">
          <PrimaryAuthButton label="Kembali ke Login" onPress={() => router.replace('/')} loading={false} />
        </View>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      title="Lupa Password"
      subtitle="Masukkan email Anda untuk mengatur ulang password">
      <AuthInput
        icon={<AtIcon size={16} color={AppColors.white} />}
        placeholder="EMAIL"
        value={email}
        onChangeText={onChangeEmail}
        autoCapitalize="none"
        keyboardType="email-address"
        onSubmitEditing={onSubmit}
      />

      <View className="mt-2.5">
        <PrimaryAuthButton label="Kirim" onPress={onSubmit} loading={loading} />
      </View>

      <Pressable onPress={() => router.replace('/')} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Ingat password? <Text className="font-bold text-white">Login</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
