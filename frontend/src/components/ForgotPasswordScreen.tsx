import { useErrorToast } from '@/components/ui/Toast';
import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AuthLayout } from '@/components/auth/AuthLayout';
import { AuthInput } from '@/components/auth/AuthInput';
import { PrimaryAuthButton } from '@/components/auth/AuthButtons';
import { AtIcon } from '@/components/ui/Icon';

export type ForgotPasswordScreenProps = {
  email: string;
  onChangeEmail: (value: string) => void;
  onSubmit: () => void;
  loading: boolean;
  errorMessage?: string;
  /** Terisi setelah permintaan diterima: form diganti pesan konfirmasi (netral, tidak menyebut email terdaftar). */
  sentMessage?: string;
};

export default function ForgotPasswordScreen({
  email,
  onChangeEmail,
  onSubmit,
  loading,
  errorMessage,
  sentMessage,
}: ForgotPasswordScreenProps) {
  useErrorToast(errorMessage);

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
