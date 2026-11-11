/**
 * LinkGoogleScreen: layar konfirmasi password untuk menautkan akun Google ke akun yang sudah terdaftar.
 *
 * Dipakai oleh : app/link-google.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik (guest)
 * Catatan      : Tampil bila email akun Google sudah terdaftar dengan password. Akun TIDAK ditautkan otomatis: pemilik
 *                email harus membuktikan password-nya (mencegah pengambilalihan akun lewat email). Logika submit dan
 *                navigasi ada di route.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { Pressable, Text, View } from 'react-native';

import { AuthInput } from '@/components/auth/AuthInput';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { LockIcon } from '@/components/ui/Icon';
import { AppColors } from '@/constants/app-theme';

/** Props layar penautan akun Google. */
export type LinkGoogleScreenProps = {
  /** Password akun terdaftar yang diisi pengguna. */
  password: string;
  /** Dipanggil setiap password berubah, dengan nilai terbaru. */
  onChangePassword: (value: string) => void;
  /** Dipanggil saat tombol Tautkan ditekan; pemanggilan API dilakukan oleh route. */
  onSubmit: () => void;
  /** Dipanggil saat pengguna membatalkan (kembali ke halaman login). */
  onCancel: () => void;
  /** true selama permintaan berjalan; tombol Tautkan menampilkan status loading. */
  loading: boolean;
  /** Pesan galat dari route, ditampilkan sebagai toast. Opsional. */
  errorMessage?: string;
};

/** Form satu field (password) untuk menautkan akun Google. */
export default function LinkGoogleScreen({
  password,
  onChangePassword,
  onSubmit,
  onCancel,
  loading,
  errorMessage,
}: LinkGoogleScreenProps) {
  // Tampilkan galat dari route sebagai toast tiap pesan berubah.
  useErrorToast(errorMessage);
  return (
    <AuthLayout title="Tautkan Akun Google" subtitle="Konfirmasi password untuk melanjutkan">
      <Text className="text-center text-[13px] leading-5 text-white">
        Email ini sudah terdaftar. Masukkan password akun Anda untuk menautkan akun Google.
      </Text>
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
        autoCapitalize="none"
        autoCorrect={false}
      />
      <View className="mt-2.5">
        <PrimaryAuthButton label="Tautkan" onPress={onSubmit} loading={loading} />
      </View>
      <Pressable onPress={onCancel} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">Batal</Text>
      </Pressable>
    </AuthLayout>
  );
}
