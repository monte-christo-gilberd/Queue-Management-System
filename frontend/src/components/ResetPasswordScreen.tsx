/**
 * ResetPasswordScreen: layar membuat password baru dari tautan reset.
 *
 * Dipakai oleh : app/reset-password.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik
 * Catatan      : subtitle menyebut minimal 8 karakter; pastikan sama dengan validasi di route. Mode 'done' menyebut sesi perangkat lain dikeluarkan.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { LockIcon } from '@/components/ui/Icon';

export type ResetPasswordScreenProps = {
  /** form: isi password baru; invalid: tautan tidak bisa dipakai; done: password sudah diganti. */
  mode: 'form' | 'invalid' | 'done';
  /** Password baru yang diisi pengguna; field ditampilkan dengan `secureTextEntry`. */
  password: string;
  /** Isi field konfirmasi password, dicocokkan dengan `password` oleh route. */
  confirmPassword: string;
  /** Dipanggil setiap password baru berubah, dengan nilai terbaru. */
  onChangePassword: (value: string) => void;
  /** Dipanggil setiap konfirmasi password berubah, dengan nilai terbaru. */
  onChangeConfirmPassword: (value: string) => void;
  /** Pesan galat konfirmasi, tampil sebagai teks di bawah field konfirmasi (bukan toast). Opsional. */
  confirmPasswordError?: string;
  /** Dipanggil saat tombol Simpan Password ditekan atau Enter di field konfirmasi; hanya dipakai di mode form. */
  onSubmit: () => void;
  /** true selama permintaan penyimpanan berjalan; tombol Simpan Password menampilkan status loading. */
  loading: boolean;
  /** Pesan galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
};

/** Layar reset password dengan tiga keadaan: form, tautan tidak berlaku, dan password sudah diganti. */
export default function ResetPasswordScreen({
  mode,
  password,
  confirmPassword,
  onChangePassword,
  onChangeConfirmPassword,
  confirmPasswordError,
  onSubmit,
  loading,
  errorMessage,
}: ResetPasswordScreenProps) {
  useErrorToast(errorMessage);

  if (mode === 'invalid') {
    return (
      <AuthLayout title="Tautan Tidak Berlaku" subtitle="Tautan salah, kedaluwarsa, atau sudah dipakai">
        <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
          Minta tautan baru untuk membuat password Anda.
        </Text>
        <View className="mt-2.5">
          <PrimaryAuthButton label="Minta Tautan Baru" onPress={() => router.replace('/forgot-password')} loading={false} />
        </View>
      </AuthLayout>
    );
  }

  if (mode === 'done') {
    return (
      <AuthLayout title="Password Diganti" subtitle="Masuk dengan password baru Anda">
        <Text accessibilityRole="alert" className="text-center text-[13px] leading-5 text-white">
          Password berhasil diganti. Sesi di perangkat lain sudah dikeluarkan.
        </Text>
        <View className="mt-2.5">
          <PrimaryAuthButton label="Ke Halaman Login" onPress={() => router.replace('/')} loading={false} />
        </View>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Buat Password Baru" subtitle="Minimal 8 karakter">
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD BARU"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
      />
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="KONFIRMASI PASSWORD"
        value={confirmPassword}
        onChangeText={onChangeConfirmPassword}
        secureTextEntry
        showPasswordToggle
        onSubmitEditing={onSubmit}
      />
      {confirmPasswordError ? <Text className="text-[12px] text-white">{confirmPasswordError}</Text> : null}

      <View className="mt-2.5">
        <PrimaryAuthButton label="Simpan Password" onPress={onSubmit} loading={loading} />
      </View>
    </AuthLayout>
  );
}
