import { Image } from 'expo-image';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';

// eslint-disable-next-line @typescript-eslint/no-require-imports
const GOOGLE_G = require('../../../assets/images/google-g.svg');

type PrimaryAuthButtonProps = {
  label: string;
  onPress?: () => void;
  loading?: boolean;
};

/** Tombol utama: persegi membulat SOLID PUTIH, teks tebal warna aksen biru. */
export function PrimaryAuthButton({ label, onPress, loading = false }: PrimaryAuthButtonProps) {
  return (
    <Pressable
      onPress={onPress}
      disabled={loading}
      className="h-12 items-center justify-center rounded-[6px] bg-white active:opacity-[0.85]">
      {loading ? (
        <ActivityIndicator color={AppColors.primary} />
      ) : (
        <Text className="text-[13px] font-extrabold uppercase tracking-[1px] text-primary">
          {label}
        </Text>
      )}
    </Pressable>
  );
}

/** Pemisah tipis dengan tulisan "atau" di tengah. */
export function OrDivider() {
  return (
    <View className="flex-row items-center gap-3">
      <View className="h-px flex-1 bg-white opacity-30" />
      <Text className="text-[12px] text-placeholder">atau</Text>
      <View className="h-px flex-1 bg-white opacity-30" />
    </View>
  );
}

type GoogleButtonProps = {
  label?: string;
  onPress?: () => void;
};

/**
 * Tombol Google: sengaja TIDAK mengikuti tema aplikasi (latar putih, border abu tipis,
 * logo "G" resmi, teks abu gelap) sesuai pedoman branding Google Sign-In.
 */
export function GoogleButton({ label = 'Login with Google', onPress }: GoogleButtonProps) {
  return (
    <Pressable
      onPress={onPress}
      className="h-12 flex-row items-center justify-center gap-2.5 rounded-[6px] border border-border bg-white active:opacity-[0.85]">
      {/* expo-image tidak dibungkus NativeWind → ukuran lewat style. */}
      <Image source={GOOGLE_G} style={styles.googleLogo} contentFit="contain" />
      <Text className="text-[14px] font-semibold text-textMuted">{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  googleLogo: { width: 18, height: 18 },
});
