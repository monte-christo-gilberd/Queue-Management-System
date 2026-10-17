import { router, type Href } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { GoogleButton, OrDivider, PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { AtIcon, LockIcon, PersonIcon } from '@/components/ui/Icon';

export type RegisterScreenProps = {
  username: string;
  email: string;
  password: string;
  /** Isi field konfirmasi password (untuk mengecek kesamaan dengan `password`). */
  confirmPassword: string;
  onChangeUsername: (value: string) => void;
  onChangeEmail: (value: string) => void;
  onChangePassword: (value: string) => void;
  onChangeConfirmPassword: (value: string) => void;
  /** Pesan error di bawah field konfirmasi (mis. "Password tidak sama"); logika pengecekannya di route. */
  confirmPasswordError?: string;
  onSubmit: () => void;
  /** Placeholder visual — Google OAuth belum ada di backend. */
  onGoogleLogin?: () => void;
  /** Tujuan tautan "Login" (mis. `/?next=…`). Default `/`. */
  loginHref?: string;
  loading: boolean;
  errorMessage?: string;
};

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
        {confirmPasswordError ? (
          <Text className="text-[12px] text-live">{confirmPasswordError}</Text>
        ) : null}
      </View>

      {errorMessage ? <Text className="text-[13px] text-live">{errorMessage}</Text> : null}

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
