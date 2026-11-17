/**
 * RegisterScreen: layar pendaftaran akun baru (username, email, password, konfirmasi password).
 *
 * Dipakai oleh : app/register.tsx
 * Bergantung   : ui/Toast (useErrorToast), components/auth/* (AuthLayout, AuthInput, AuthButtons), expo-router
 * Peran        : publik
 * Catatan      : pengecekan kesamaan password dilakukan di route; komponen hanya menampilkan pesannya sebagai teks di bawah
 *                field konfirmasi (bukan toast), dan memberi tahu route saat field itu kehilangan fokus. Galat dari route
 *                (isian wajib, password lemah, galat server) tetap tampil sebagai toast.
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
import { AtIcon, LockIcon, PersonIcon } from '@/components/ui/Icon';
import { useAuthKeyboardNav } from '@/hooks/use-auth-keyboard-nav';

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
  /** Dipanggil saat field konfirmasi password kehilangan fokus; route memakainya untuk mulai menampilkan galat konfirmasi. */
  onBlurConfirmPassword?: () => void;
  /**
   * Pesan error di bawah field konfirmasi (mis. "Password tidak sama"), tampil sebagai teks, bukan toast. Logika dan
   * waktu tampilnya (setelah blur atau submit) diatur route. Kosong berarti tidak ada pesan.
   */
  confirmPasswordError?: string;
  /** Dipanggil saat tombol Daftar ditekan; validasi dan pembuatan akun dilakukan oleh route. */
  onSubmit: () => void;
  /** Tujuan setelah daftar/login Google berhasil (sudah disanitasi). Kosong = /dashboard. */
  next?: string;
  /** Tujuan tautan "Login" (mis. `/login?next=…`). Default `/login`. */
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
  onBlurConfirmPassword,
  confirmPasswordError,
  onSubmit,
  next,
  loginHref = '/login',
  loading,
  errorMessage,
}: RegisterScreenProps) {
  // Hanya galat dari route yang menjadi toast. Galat konfirmasi password tampil inline di bawah field, karena toast
  // yang muncul sejak karakter pertama mengganggu pengguna yang masih mengetik.
  useErrorToast(errorMessage);

  // Urutan fokus dari atas ke bawah: username, email, password, konfirmasi, Daftar, Google.
  const usernameRef = useRef<TextInput>(null);
  const emailRef = useRef<TextInput>(null);
  const passwordRef = useRef<TextInput>(null);
  const confirmRef = useRef<TextInput>(null);
  const submitRef = useRef<View>(null);
  const googleRef = useRef<View>(null);
  useAuthKeyboardNav([usernameRef, emailRef, passwordRef, confirmRef, submitRef, googleRef]);

  // Empat field berurutan, tombol Daftar, pendaftaran dengan Google, lalu tautan kembali ke login.
  // Enter di field mana pun langsung mengirim form (route yang memvalidasi isian yang belum lengkap).
  return (
    <AuthLayout title="Buat Akun" subtitle="Daftar untuk mulai mengelola antrian Anda">
      <AuthInput
        ref={usernameRef}
        icon={<PersonIcon size={16} color={AppColors.white} />}
        placeholder="USERNAME"
        value={username}
        onChangeText={onChangeUsername}
        autoCapitalize="none"
        returnKeyType="go"
        onSubmitEditing={onSubmit}
      />
      <AuthInput
        ref={emailRef}
        icon={<AtIcon size={16} color={AppColors.white} />}
        placeholder="EMAIL"
        value={email}
        onChangeText={onChangeEmail}
        autoCapitalize="none"
        keyboardType="email-address"
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
      <View className="gap-1.5">
        <AuthInput
          ref={confirmRef}
          icon={<LockIcon size={16} color={AppColors.white} />}
          placeholder="KONFIRMASI PASSWORD"
          value={confirmPassword}
          onChangeText={onChangeConfirmPassword}
          onBlur={onBlurConfirmPassword}
          secureTextEntry
          showPasswordToggle
          returnKeyType="go"
          onSubmitEditing={onSubmit}
        />
        {/* Pesan inline, bukan toast: tampil setelah blur atau submit dan hilang saat isinya sudah sama. */}
        {confirmPasswordError ? <Text className="text-[12px] text-white">{confirmPasswordError}</Text> : null}
      </View>

      <View className="mt-2.5">
        <PrimaryAuthButton buttonRef={submitRef} label="Daftar" onPress={onSubmit} loading={loading} />
      </View>

      {/* Jalur alternatif tanpa password; next diteruskan agar pengguna kembali ke halaman asal setelah masuk. */}
      <GoogleSignInSection label="Daftar dengan Google" next={next} buttonRef={googleRef} />

      {/* replace (bukan push) supaya tombol kembali tidak membawa pengguna lagi ke form daftar. */}
      <Pressable onPress={() => router.replace(loginHref as Href)} className="mt-2 items-center">
        <Text className="text-[12px] text-placeholder">
          Sudah punya akun? <Text className="font-bold text-white">Login</Text>
        </Text>
      </Pressable>
    </AuthLayout>
  );
}
