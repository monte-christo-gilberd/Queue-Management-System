/**
 * Profile: halaman profil user (nama, email, foto, notifikasi, password, dan status langganan untuk owner).
 *
 * Dipakai oleh : app/(user)/profile/index.tsx
 * Bergantung   : layout/ResponsiveScreenLayout, ui/ConfirmModal, ui/ModalCard, ui/Toast, ui/AppButton, ui/FormField, ui/Icon,
 *                hooks/use-avatar-source (tipe), hooks/use-is-wide, utils/profile (USERNAME_MAX), utils/avatar-preview,
 *                utils/cn, constants/app-theme, expo-image, expo-router
 * Peran        : owner dan customer (kartu langganan hanya untuk owner; ajakan "Jadi Owner" untuk user biasa)
 * Catatan      : banner verifikasi email hanya tampil bila `emailVerified` false; tombol simpan dan validasi nama ada di route.
 *                Foto profil di kartu kiri bisa diketuk untuk melihatnya diperbesar di popup; tanpa foto tidak bisa diketuk.
 */
import { Image } from 'expo-image';
import { useState } from 'react';

import { ConfirmModal } from '@/components/ui/ConfirmModal';
import { useErrorToast } from '@/components/ui/Toast';
import type { AvatarSource } from '@/hooks/use-avatar-source';
import { USERNAME_MAX } from '@/utils/profile';
import { router } from 'expo-router';
import { Pressable, Switch, Text, useWindowDimensions, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useIsWide } from '@/hooks/use-is-wide';
import { getAvatarPreviewSize } from '@/utils/avatar-preview';
import { cn } from '@/utils/cn';
import ResponsiveScreenLayout, {
  type MenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { PersonIcon } from '@/components/ui/Icon';
import { ModalCard } from '@/components/ui/ModalCard';

/** Props halaman profil user. */
export type ProfileProps = {
  /** Konteks shell layout; hanya `userName` yang dipakai untuk header. Opsional. */
  shell?: ShellProps;
  /** Dipanggil saat item menu owner ditekan; opsional, tanpa nilai menu tidak ada aksi. */
  onMenuItemPress?: (item: MenuItem) => void;
  /** Nilai nama user yang sedang diedit; dibatasi panjang maksimum `USERNAME_MAX`. */
  username: string;
  /** Email hanya ditampilkan; mengubahnya belum didukung (verifikasi ganti email belum dibangun). */
  email: string;
  /** false = email belum diverifikasi: tampilkan banner dan tombol kirim ulang. Bawaan true (tidak ada banner). */
  emailVerified?: boolean;
  /** Minta tautan verifikasi dikirim ulang ke email akun. */
  onPressResendVerification?: () => void;
  /** Permintaan kirim ulang sedang berjalan. */
  resendVerificationBusy?: boolean;
  /** Dipanggil setiap nama user berubah, dengan nilai terbaru. */
  onChangeUsername: (value: string) => void;
  /** Dipanggil saat tombol "Simpan Perubahan" ditekan; penyimpanan dilakukan oleh route. */
  onSubmit: () => void;
  /** Tombol simpan nonaktif bila nama tidak berubah atau tidak valid. */
  canSave: boolean;
  /** true saat penyimpanan profil berjalan; tombol simpan menampilkan status loading. */
  loading: boolean;
  /** Galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
  /** Pesan sukses di bawah tombol simpan (mis. "Perubahan tersimpan"). */
  savedMessage?: string;
  /** true bila user adalah owner; hanya owner yang melihat kartu status langganan. */
  isOwner: boolean;
  /** User biasa (profil sudah dimuat, bukan owner): tampilkan ajakan "Jadi Owner" ke halaman Subscription. */
  canBecomeOwner?: boolean;
  /** Status langganan owner: true berarti aktif (badge "Aktif"), false berarti belum aktif atau sudah habis (badge "Belum Bayar"). Hanya dipakai bila `isOwner`. Default false. */
  subscriptionPaid?: boolean;
  /** Keterangan di bawah status dari route, mis. "Aktif hingga 1 Feb 2025". */
  subscriptionDescription?: string;
  /** Sumber foto profil untuk tampilan kecil dan popup; null atau kosong berarti tampilkan ikon orang. */
  avatarSource?: AvatarSource | null;
  /** Server menyimpan foto untuk user ini (menentukan tombol Hapus Foto). */
  hasAvatar?: boolean;
  /** Unggah atau hapus foto sedang berjalan. */
  avatarBusy?: boolean;
  /** Dipanggil saat tombol Unggah atau Ganti Foto ditekan; pemilihan berkas dilakukan route. Opsional. */
  onPickAvatar?: () => void;
  /** Dipanggil setelah user mengonfirmasi hapus foto. */
  onDeleteAvatar?: () => void;
  /** Nilai switch notifikasi email tiket (dari server). Default true. */
  emailNotifications?: boolean;
  /** true saat perubahan notifikasi email sedang disimpan; switch dinonaktifkan. Default false. */
  emailNotificationsBusy?: boolean;
  /** Dipanggil dengan nilai baru saat switch notifikasi email diubah. Opsional. */
  onToggleEmailNotifications?: (enabled: boolean) => void;
  /** Minta tautan buat password baru dikirim ke email akun (item 9). */
  onPressChangePassword?: () => void;
  /** Permintaan tautan sedang berjalan. */
  changePasswordBusy?: boolean;
};

/** Halaman profil user: foto, nama, email, notifikasi, password, dan status langganan (owner). */
export default function Profile({
  shell,
  onMenuItemPress,
  username,
  email,
  emailVerified = true,
  onPressResendVerification,
  resendVerificationBusy = false,
  onChangeUsername,
  onSubmit,
  canSave,
  loading,
  errorMessage,
  savedMessage,
  isOwner,
  canBecomeOwner = false,
  subscriptionPaid = false,
  subscriptionDescription,
  onPressChangePassword,
  changePasswordBusy = false,
  emailNotifications = true,
  emailNotificationsBusy = false,
  onToggleEmailNotifications,
  avatarSource = null,
  hasAvatar = false,
  avatarBusy = false,
  onPickAvatar,
  onDeleteAvatar,
}: ProfileProps) {
  // Galat dari route ditampilkan sebagai toast.
  useErrorToast(errorMessage);
  // Hapus foto selalu lewat konfirmasi; onDeleteAvatar baru dipanggil setelah pengguna memastikan.
  const [confirmDelete, setConfirmDelete] = useState(false);
  // true saat popup foto yang diperbesar terbuka; hanya bisa dibuka bila foto ada (lihat avatar di bawah).
  const [avatarPreviewOpen, setAvatarPreviewOpen] = useState(false);
  // Layar lebar memakai dua kolom (lihat JSX).
  const isWide = useIsWide();
  // Sisi foto besar dihitung dari ukuran jendela agar selalu muat di dalam popup, di layar sempit maupun pendek.
  const { width: windowWidth, height: windowHeight } = useWindowDimensions();
  const avatarPreviewSize = getAvatarPreviewSize(windowWidth, windowHeight);

  // Layar lebar: kolom kiri (foto dan status) dan kanan (form); layar sempit: satu kolom bertumpuk.
  return (
    <ResponsiveScreenLayout
      title="Profil"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="profile">
      <View className={isWide ? 'flex-row items-start gap-6' : 'gap-4'}>
        <View className={cn('gap-4', isWide && 'w-80')}>
          {/* Kartu foto profil: tampil untuk semua user yang login. Tanpa foto, tampil ikon. */}
          <View
            className={cn(
              'items-center gap-3 rounded-card bg-surface',
              isWide ? 'w-80 py-8' : 'py-3.5',
            )}>
            {/* Bila foto ada, avatar bisa diketuk untuk melihat foto yang diperbesar. Tanpa foto (hanya ikon) tidak ada yang diperbesar. */}
            {avatarSource ? (
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="Perbesar foto profil"
                onPress={() => setAvatarPreviewOpen(true)}
                className="h-[88px] w-[88px] items-center justify-center overflow-hidden rounded-full bg-dark">
                <Image
                  source={avatarSource}
                  style={{ width: 88, height: 88 }}
                  contentFit="cover"
                  accessibilityLabel="Foto profil"
                />
              </Pressable>
            ) : (
              <View className="h-[88px] w-[88px] items-center justify-center overflow-hidden rounded-full bg-dark">
                <PersonIcon size={40} color={AppColors.white} />
              </View>
            )}
            {/* Tombol foto: Unggah atau Ganti memilih berkas lewat route; Hapus hanya bila server menyimpan foto. */}
            <View className="flex-row gap-2">
              <AppButton
                label={hasAvatar ? 'Ganti Foto' : 'Unggah Foto'}
                variant="outline"
                onPress={onPickAvatar}
                loading={avatarBusy}
                className="h-10 px-4"
              />
              {hasAvatar ? (
                <AppButton
                  label="Hapus Foto"
                  variant="outline"
                  onPress={() => setConfirmDelete(true)}
                  disabled={avatarBusy}
                  className="h-10 px-4"
                />
              ) : null}
            </View>
          </View>

          {/* Status subscription: hanya untuk owner. Link ke halaman Subscription selalu ada; di sana owner menambah masa langganan 30 hari (tanpa pembayaran). */}
          {isOwner ? (
            <View className="gap-2 rounded-card border border-line p-4">
              <View className="flex-row items-center justify-between">
                <Text className="text-[14px] font-bold text-foreground">Status Subscription</Text>
                <View
                  className={cn(
                    'rounded-pill px-3 py-1',
                    subscriptionPaid ? 'bg-primary' : 'bg-disabled',
                  )}>
                  <Text
                    className={cn(
                      'text-[11px] font-extrabold',
                      subscriptionPaid ? 'text-white' : 'text-foreground',
                    )}>
                    {subscriptionPaid ? 'Aktif' : 'Belum Bayar'}
                  </Text>
                </View>
              </View>
              <Text className="text-[12px] font-normal text-muted">
                {subscriptionDescription ??
                  (subscriptionPaid
                    ? 'Langganan Anda aktif.'
                    : 'Anda belum berlangganan atau belum membayar tagihan bulan ini.')}
              </Text>
              <Pressable onPress={() => router.push('/subscription')} accessibilityRole="link">
                <Text className="text-[13px] font-bold text-primary">
                  {subscriptionPaid ? 'Perpanjang Langganan →' : 'Lihat Subscription & Bayar →'}
                </Text>
              </Pressable>
            </View>
          ) : null}

          {/* Ajakan Jadi Owner: hanya untuk user biasa yang profilnya sudah dimuat (canBecomeOwner). Di halaman Subscription, 30 hari pertama langsung menjadikan akun owner (tanpa pembayaran). */}
          {canBecomeOwner ? (
            <View className="gap-2 rounded-card border border-line p-4">
              <Text className="text-[14px] font-bold text-foreground">Jadi Owner</Text>
              <Text className="text-[12px] font-normal text-muted">
                Ingin membuat dan mengelola antrian sendiri? Berlangganan 1 bulan untuk menjadi owner.
              </Text>
              <Pressable onPress={() => router.push('/subscription')} accessibilityRole="link" accessibilityLabel="Jadi Owner">
                <Text className="text-[13px] font-bold text-primary">Jadi Owner & Bayar →</Text>
              </Pressable>
            </View>
          ) : null}
        </View>

        {/* Kolom kanan: form nama dan email, banner verifikasi, simpan, notifikasi, dan password. */}
        <View className={cn('gap-4', isWide && 'flex-1')}>
          {/* Form: nama bisa diubah; email hanya tampil (editable false). */}
          <FormField
            label="Nama User"
            value={username}
            onChangeText={onChangeUsername}
            placeholder="Nama User"
            maxLength={USERNAME_MAX}
          />
          <FormField label="Email User" value={email} editable={false} placeholder="Email User" />

          {/* Banner verifikasi: hanya tampil bila emailVerified false (profil sudah dimuat dan email belum terverifikasi). Tombol meminta kirim ulang tautan. */}
          {emailVerified ? null : (
            <View
              accessibilityRole="alert"
              accessibilityLabel="Email belum diverifikasi"
              className="gap-2 rounded-card border border-line bg-surface p-4">
              <Text className="text-[14px] font-bold text-foreground">Email belum diverifikasi</Text>
              <Text className="text-[12px] font-normal text-muted">
                Verifikasi email Anda lewat tautan yang kami kirim agar bisa bergabung ke antrian dan membuat antrian.
                Periksa juga folder spam.
              </Text>
              <AppButton
                label="Kirim Ulang Email Verifikasi"
                variant="outline"
                onPress={onPressResendVerification}
                loading={resendVerificationBusy}
                disabled={resendVerificationBusy}
              />
            </View>
          )}

          {/* Tombol simpan: aktif hanya bila canSave; penyimpanan dilakukan route. */}
          <AppButton
            label="Simpan Perubahan"
            variant="dark"
            onPress={onSubmit}
            loading={loading}
            disabled={!canSave}
          />
          {/* Pesan sukses di bawah tombol; hanya tampil bila route mengisi savedMessage. */}
          {savedMessage ? (
            <Text className="text-[13px] font-semibold text-primary">{savedMessage}</Text>
          ) : null}

          {/* Notifikasi email: switch untuk menyalakan atau mematikan email tiket; dikunci selama menyimpan. */}
          <View className="gap-2 rounded-card border border-line p-4">
            <View className="flex-row items-center justify-between gap-3">
              <View className="flex-1 gap-1">
                <Text className="text-[14px] font-bold text-foreground">Notifikasi email</Text>
                <Text className="text-[12px] font-normal text-muted">
                  Kirim email saat Anda masuk antrian, membatalkan, nomor dihapus penyelenggara, atau selesai dilayani.
                </Text>
              </View>
              <Switch
                accessibilityLabel="Notifikasi email"
                value={emailNotifications}
                disabled={emailNotificationsBusy}
                onValueChange={onToggleEmailNotifications}
                trackColor={{ true: AppColors.primary }}
              />
            </View>
          </View>

          {/* Password: tidak ada form di sini; tombol meminta tautan buat password baru dikirim ke email akun. */}
          <View className="gap-2 rounded-card border border-line p-4">
            <Text className="text-[14px] font-bold text-foreground">Password</Text>
            <Text className="text-[12px] font-normal text-muted">
              Tautan untuk membuat password baru akan dikirim ke email akun Anda.
            </Text>
            <AppButton
              label="Minta Ganti Password"
              variant="outline"
              onPress={onPressChangePassword}
              loading={changePasswordBusy}
              disabled={changePasswordBusy}
            />
          </View>
        </View>
      </View>
      {/* Konfirmasi hapus foto: terbuka dari tombol Hapus Foto; onDeleteAvatar baru jalan setelah "Ya, Hapus". */}
      <ConfirmModal
        visible={confirmDelete}
        title="Hapus foto profil?"
        message="Foto profil Anda akan dihapus dan diganti ikon. Anda bisa mengunggah foto lagi kapan saja."
        confirmLabel="Ya, Hapus"
        loading={avatarBusy}
        onConfirm={() => {
          setConfirmDelete(false);
          onDeleteAvatar?.();
        }}
        onCancel={() => setConfirmDelete(false)}
      />
      {/* Popup foto diperbesar: terbuka dengan mengetuk avatar; ditutup dengan tombol X di pojok kanan atas, tap di luar kartu, atau tombol back perangkat. */}
      {/* compact: bingkai putih di sekeliling foto hanya 8 px; ukuran foto dihitung getAvatarPreviewSize dengan angka yang sama. */}
      <ModalCard visible={avatarPreviewOpen} onRequestClose={() => setAvatarPreviewOpen(false)} compact>
        {/* Isi hanya dirender bila foto ada, agar popup tidak kosong saat foto dihapus. */}
        {avatarSource ? (
          <View className="items-center">
            {/* Pembungkus selebar foto supaya tombol X menempel di pojok foto, bukan di tepi kartu. */}
            <View style={{ width: avatarPreviewSize, height: avatarPreviewSize }}>
              {/* Foto tersimpan persegi 512x512, jadi ditampilkan utuh (bukan lingkaran) dengan sudut membulat. */}
              <View
                className="overflow-hidden rounded-card bg-dark"
                style={{ width: avatarPreviewSize, height: avatarPreviewSize }}>
                <Image
                  source={avatarSource}
                  style={{ width: avatarPreviewSize, height: avatarPreviewSize }}
                  contentFit="cover"
                  accessibilityLabel="Foto profil diperbesar"
                />
              </View>
              {/* Latar gelap semi-transparan agar tanda X tetap terbaca di atas foto yang terang. */}
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="Tutup foto"
                onPress={() => setAvatarPreviewOpen(false)}
                hitSlop={8}
                className="absolute right-2 top-2 h-8 w-8 items-center justify-center rounded-full bg-modalBackdrop">
                <Text className="text-[20px] font-bold leading-[20px] text-white">×</Text>
              </Pressable>
            </View>
          </View>
        ) : null}
      </ModalCard>
    </ResponsiveScreenLayout>
  );
}
