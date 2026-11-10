/**
 * RegisterScreen: layar pendaftaran akun baru (username, email, password, konfirmasi password).
 *
 * Dipakai oleh : app/register.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik
 * Catatan      : pengecekan kesamaan password dilakukan di route; komponen hanya menampilkan pesannya.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { router, type Href } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { GoogleButton, OrDivider, PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { AtIcon, LockIcon, PersonIcon } from '@/components/ui/Icon';

/** Props layar pendaftaran akun. */
export type RegisterScreenProps = {
  /** Nilai username saat ini. */
  username: string;
  /** Nilai email saat ini; keyboard email ditampilkan untuk field ini. */
  email: string;
  /** Nilai password saat ini; field ditampilkan dengan `secureTextEntry`. */
  password: string;
  /** Isi field konfirmasi password (untuk mengecek kesamaan dengan `password`). */
  confirmPassword: string;
  /** Dipanggil setiap username berubah, dengan nilai terbaru. */
  onChangeUsername: (value: string) => void;
  /** Dipanggil setiap email berubah, dengan nilai terbaru. */
  onChangeEmail: (value: string) => void;
  /** Dipanggil setiap password berubah, dengan nilai terbaru. */
  onChangePassword: (value: string) => void;
  /** Dipanggil setiap konfirmasi password berubah, dengan nilai terbaru. */
  onChangeConfirmPassword: (value: string) => void;
  /** Pesan error di bawah field konfirmasi (mis. "Password tidak sama"); logika pengecekannya di route. */
  confirmPasswordError?: string;
  /** Dipanggil saat tombol Daftar ditekan; validasi dan pembuatan akun dilakukan oleh route. */
  onSubmit: () => void;
  /** Placeholder visual — Google OAuth belum ada di backend. */
  onGoogleLogin?: () => void;
  /** Tujuan tautan "Login" (mis. `/?next=…`). Default `/`. */
  loginHref?: string;
  /** true selama permintaan pendaftaran berjalan; tombol Daftar menampilkan status loading. */
  loading: boolean;
  /** Pesan galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
};

/** Layar pendaftaran dengan empat field, tombol Daftar, dan tautan kembali ke login. */
export default function RegisterScreen({
  username,
  email,
  password,
  confirmPassword,
  onChangeUsername,
  onChangeEmail,
  onChangePassword,
  onChangeConfirmPassword,
  confirmPasswordError,
  onSubmit,
  onGoogleLogin,
  loginHref = '/',
  loading,
  errorMessage,
}: RegisterScreenProps) {
  // Pesan error konfirmasi password dan galat dari route sama-sama tampil sebagai toast.
  useErrorToast(confirmPasswordError);
  useErrorToast(errorMessage);
  return (
    <AuthLayout title="Buat Akun" subtitle="Daftar untuk mulai mengelola antrian Anda">
      <AuthInput
        icon={<PersonIcon size={16} color={AppColors.white} />}
        placeholder="USERNAME"
        value={username}
        onChangeText={onChangeUsername}
        autoCapitalize="none"
      />
      <AuthInput
        icon={<AtIcon size={16} color={AppColors.white} />}
        placeholder="EMAIL"
        value={email}
        onChangeText={onChangeEmail}
        autoCapitalize="none"
        keyboardType="email-address"
      />
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
      />
      <View className="gap-1.5">
        <AuthInput
          icon={<LockIcon size={16} color={AppColors.white} />}
          placeholder="KONFIRMASI PASSWORD"
          value={confirmPassword}
          onChangeText={onChangeConfirmPassword}
          secureTextEntry
          showPasswordToggle
        />
      </View>

      <View className="mt-2.5">
        <PrimaryAuthButton label="Daftar" onPress={onSubmit} loading={loading} />
      </View>

      <OrDivider />

      {/* TODO: sambungkan setelah Google OAuth backend tersedia */}
      <GoogleButton label="Daftar dengan Google" onPress={onGoogleLogin} />

      <Pressable onPress={() => router.replace(loginHref as Href)} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Sudah punya akun? <Text className="font-bold text-white">Login</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
