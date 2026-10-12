import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { GoogleButton, OrDivider, PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { LockIcon, PersonIcon } from '@/components/ui/Icon';

export type LoginScreenProps = {
  username: string;
  password: string;
  onChangeUsername: (value: string) => void;
  onChangePassword: (value: string) => void;
  onSubmit: () => void;
  /** Placeholder visual — Google OAuth belum ada di backend. */
  onGoogleLogin?: () => void;
  /** Opsional: override handler "Forgot password?" (default: buka halaman /forgot-password). */
  onForgotPassword?: () => void;
  loading: boolean;
  errorMessage?: string;
};

export default function LoginScreen({
  username,
  password,
  onChangeUsername,
  onChangePassword,
  onSubmit,
  onGoogleLogin,
  onForgotPassword,
  loading,
  errorMessage,
}: LoginScreenProps) {
  return (
    <AuthLayout title="Queue Management System" subtitle="Kelola antrian Anda dari mana saja">
      {/* TODO: konfirmasi sumber data — backend login memakai email, mockup memakai username */}
      <AuthInput
        icon={<PersonIcon size={16} color={AppColors.white} />}
        placeholder="USERNAME"
        value={username}
        onChangeText={onChangeUsername}
        autoCapitalize="none"
      />
      <AuthInput
        icon={<LockIcon size={16} color={AppColors.white} />}
        placeholder="PASSWORD"
        value={password}
        onChangeText={onChangePassword}
        secureTextEntry
        showPasswordToggle
      />

      {errorMessage ? <Text className="text-[13px] text-live">{errorMessage}</Text> : null}

      <View className="mt-2.5">
        <PrimaryAuthButton label="Login" onPress={onSubmit} loading={loading} />
      </View>

      <Pressable onPress={onForgotPassword ?? (() => router.push('/forgot-password'))} className="self-end">
        <Text className="text-[12px] text-placeholder">Forgot password?</Text>
      </Pressable>

      <OrDivider />

      {/* TODO: sambungkan setelah Google OAuth backend tersedia */}
      <GoogleButton onPress={onGoogleLogin} />

      <Pressable onPress={() => router.push('/register')} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Belum punya akun? <Text className="font-bold text-white">Daftar</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
