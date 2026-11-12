/**
 * AuthButtons: kumpulan tombol dan pemisah untuk halaman auth (tombol utama, pemisah "atau", tombol Google).
 *
 * Dipakai oleh : components/LoginScreen.tsx, components/RegisterScreen.tsx, components/ForgotPasswordScreen.tsx,
 *                components/ResetPasswordScreen.tsx, components/VerifyEmailScreen.tsx, components/LinkGoogleScreen.tsx,
 *                components/auth/GoogleSignInSection.tsx
 * Bergantung   : expo-image, constants/app-theme, assets/images/google-g.svg
 * Catatan      : Logo G dimuat lewat require (SVG sebagai modul).
 */
import { Image } from 'expo-image';
import type { Ref } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';

// Logo Google "G" (SVG sebagai modul); require dipakai karena bundler memuat aset ini sebagai modul.
// eslint-disable-next-line @typescript-eslint/no-require-imports
const GOOGLE_G = require('../../../assets/images/google-g.svg');

/** Props tombol utama auth. */
type PrimaryAuthButtonProps = {
  /** Teks tombol; tampil dalam huruf kapital lewat gaya. */
  label: string;
  /** Dipanggil saat ditekan. Tidak dipanggil saat loading. */
  onPress?: () => void;
  /** Bila true, tombol terkunci dan menampilkan spinner. Default false. */
  loading?: boolean;
  /** Ref ke elemen tombol (untuk navigasi fokus dengan keyboard). */
  buttonRef?: Ref<View>;
};

/** Tombol utama: persegi membulat SOLID PUTIH, teks tebal warna aksen biru. */
export function PrimaryAuthButton({ label, onPress, loading = false, buttonRef }: PrimaryAuthButtonProps) {
  // disabled saat loading mencegah submit ganda; spinner menggantikan teks supaya lebar tombol tetap.
  return (
    <Pressable
      ref={buttonRef}
      onPress={onPress}
      disabled={loading}
      accessibilityState={{ disabled: loading }}
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

/** Props tombol Google. */
type GoogleButtonProps = {
  /** Teks tombol. Default "Login with Google". */
  label?: string;
  /** Dipanggil saat tombol ditekan. */
  onPress?: () => void;
  /** Bila true, tombol terkunci dan diredupkan (mis. permintaan belum siap atau login sedang berjalan). */
  disabled?: boolean;
  /** Ref ke elemen tombol (untuk navigasi fokus dengan keyboard). */
  buttonRef?: Ref<View>;
};

/**
 * Tombol Google: sengaja TIDAK mengikuti tema aplikasi (latar putih, border abu tipis,
 * logo "G" resmi, teks abu gelap) sesuai pedoman branding Google Sign-In.
 */
export function GoogleButton({ label = 'Login with Google', onPress, disabled = false, buttonRef }: GoogleButtonProps) {
  // accessibilityState memberi tahu pembaca layar bahwa tombol terkunci; opacity hanya penanda visual.
  return (
    <Pressable
      ref={buttonRef}
      onPress={onPress}
      disabled={disabled}
      accessibilityState={{ disabled }}
      className={`h-12 flex-row items-center justify-center gap-2.5 rounded-[6px] border border-border bg-white active:opacity-[0.85] ${disabled ? 'opacity-60' : ''}`}>
      {/* expo-image tidak dibungkus NativeWind → ukuran lewat style. */}
      <Image source={GOOGLE_G} style={styles.googleLogo} contentFit="contain" />
      <Text className="text-[14px] font-semibold text-textMuted">{label}</Text>
    </Pressable>
  );
}

/** Gaya untuk komponen yang tidak lewat NativeWind (expo-image). */
const styles = StyleSheet.create({
  googleLogo: { width: 18, height: 18 },
});
