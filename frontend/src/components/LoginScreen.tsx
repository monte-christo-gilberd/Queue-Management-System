/**
 * LoginScreen: layar login dengan email atau username dan password.
 *
 * Dipakai oleh : app/index.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik
 * Catatan      : tombol Google hanya placeholder sampai OAuth tersedia di backend; logika submit dan navigasi ada di route.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { router, type Href } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { GoogleButton, OrDivider, PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { LockIcon, PersonIcon } from '@/components/ui/Icon';

export type LoginScreenProps = {
  /** Email atau username. Backend menerima keduanya lewat field `identifier`. */
  identifier: string;
  /** Nilai password saat ini; field ditampilkan dengan `secureTextEntry`. */
  password: string;
  /** Dipanggil setiap teks di field email/username berubah, dengan nilai terbaru. */
  onChangeIdentifier: (value: string) => void;
  /** Dipanggil setiap teks di field password berubah, dengan nilai terbaru. */
  onChangePassword: (value: string) => void;
  /** Dipanggil saat tombol Login ditekan; validasi dan pemanggilan API dilakukan oleh route. */
  onSubmit: () => void;
  /** Placeholder visual — Google OAuth belum ada di backend. */
  onGoogleLogin?: () => void;
  /** Opsional: override handler "Forgot password?" (default: buka halaman /forgot-password). */
  onForgotPassword?: () => void;
  /** Tujuan tautan "Daftar" (mis. `/register?next=…`). Default `/register`. */
  registerHref?: string;
  /** true selama permintaan login berjalan; tombol Login menampilkan status loading. */
  loading: boolean;
  /** Pesan galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
};

/** Layar login: field identifier dan password, tombol Login, lupa password, dan tautan ke halaman daftar. */
export default function LoginScreen({
  identifier,
  password,
  onChangeIdentifier,
  onChangePassword,
  onSubmit,
  onGoogleLogin,
  onForgotPassword,
  registerHref = '/register',
  loading,
  errorMessage,
}: LoginScreenProps) {
  useErrorToast(errorMessage);
  return (
    <AuthLayout title="Queue Management System" subtitle="Kelola antrian Anda dari mana saja">
      <AuthInput
        icon={<PersonIcon size={16} color={AppColors.white} />}
        placeholder="EMAIL ATAU USERNAME"
        value={identifier}
        onChangeText={onChangeIdentifier}
        autoCapitalize="none"
        autoCorrect={false}
      />
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
      />

      <View className="mt-2.5">
        <PrimaryAuthButton label="Login" onPress={onSubmit} loading={loading} />
      </View>

      <Pressable onPress={onForgotPassword ?? (() => router.push('/forgot-password'))} className="self-end">
        <Text className="text-[12px] text-placeholder">Forgot password?</Text>
      </Pressable>

      <OrDivider />

      {/* TODO: sambungkan setelah Google OAuth backend tersedia */}
      <GoogleButton onPress={onGoogleLogin} />

      <Pressable onPress={() => router.push(registerHref as Href)} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Belum punya akun? <Text className="font-bold text-white">Daftar</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
