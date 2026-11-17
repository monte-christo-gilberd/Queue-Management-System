/**
 * LoginScreen: layar login dengan email atau username dan password.
 *
 * Dipakai oleh : app/login.tsx
 * Bergantung   : ui/Toast (useErrorToast), ui/Icon, constants/app-theme, expo-router,
 *                components/auth/* (AuthLayout, AuthInput, AuthButtons, GoogleSignInSection)
 * Peran        : publik
 * Catatan      : tombol Google (GoogleSignInSection) menangani loginnya sendiri dan tersembunyi bila Google belum dikonfigurasi;
 *                logika submit dan navigasi login biasa ada di route.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { router, type Href } from 'expo-router';
import { useRef } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { GoogleSignInSection } from '@/components/auth/GoogleSignInSection';
import { LockIcon, PersonIcon } from '@/components/ui/Icon';
import { useAuthKeyboardNav } from '@/hooks/use-auth-keyboard-nav';

/** Props layar login. */
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
  /** Tujuan setelah login Google berhasil (sudah disanitasi). Kosong = /dashboard. */
  next?: string;
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
  next,
  onForgotPassword,
  registerHref = '/register',
  loading,
  errorMessage,
}: LoginScreenProps) {
  // Tampilkan galat dari route sebagai toast tiap pesan berubah.
  useErrorToast(errorMessage);

  // Urutan fokus dari atas ke bawah: email/username, password, Login, Google.
  const identifierRef = useRef<TextInput>(null);
  const passwordRef = useRef<TextInput>(null);
  const loginRef = useRef<View>(null);
  const googleRef = useRef<View>(null);
  useAuthKeyboardNav([identifierRef, passwordRef, loginRef, googleRef]);

  return (
    <AuthLayout title="Queue Management System" subtitle="Kelola antrian Anda dari mana saja">
      <AuthInput
        ref={identifierRef}
        icon={<PersonIcon size={16} color={AppColors.white} />}
        placeholder="EMAIL ATAU USERNAME"
        value={identifier}
        onChangeText={onChangeIdentifier}
        autoCapitalize="none"
        autoCorrect={false}
        returnKeyType="go"
        onSubmitEditing={onSubmit}
      />
      <AuthInput
        ref={passwordRef}
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
        returnKeyType="go"
        onSubmitEditing={onSubmit}
      />

      {/* Jarak ekstra di atas tombol utama supaya terpisah dari field. */}
      <View className="mt-2.5">
        <PrimaryAuthButton buttonRef={loginRef} label="Login" onPress={onSubmit} loading={loading} />
      </View>

      {/* push (bukan replace): pengguna bisa kembali ke login dari halaman lupa password. */}
      <Pressable onPress={onForgotPassword ?? (() => router.push('/forgot-password'))} className="self-end">
        <Text className="text-[12px] text-placeholder">Forgot password?</Text>
      </Pressable>

      {/* Bagian ini tidak merender apa pun bila Google belum dikonfigurasi. */}
      <GoogleSignInSection next={next} buttonRef={googleRef} />

      <Pressable onPress={() => router.push(registerHref as Href)} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Belum punya akun? <Text className="font-bold text-white">Daftar</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
