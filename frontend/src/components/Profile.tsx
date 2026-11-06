import { Image } from 'expo-image';
import { useState } from 'react';

import { ConfirmModal } from '@/components/ui/ConfirmModal';
import { useErrorToast } from '@/components/ui/Toast';
import type { AvatarSource } from '@/hooks/use-avatar-source';
import { USERNAME_MAX } from '@/utils/profile';
import { router } from 'expo-router';
import { Pressable, Switch, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useIsWide } from '@/hooks/use-is-wide';
import { cn } from '@/utils/cn';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { PersonIcon } from '@/components/ui/Icon';

export type ProfileProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  username: string;
  /** Email hanya ditampilkan; mengubahnya belum didukung (verifikasi ganti email belum dibangun). */
  email: string;
  /** false = email belum diverifikasi: tampilkan banner dan tombol kirim ulang. Bawaan true (tidak ada banner). */
  emailVerified?: boolean;
  /** Minta tautan verifikasi dikirim ulang ke email akun. */
  onPressResendVerification?: () => void;
  /** Permintaan kirim ulang sedang berjalan. */
  resendVerificationBusy?: boolean;
  onChangeUsername: (value: string) => void;
  onSubmit: () => void;
  /** Tombol simpan nonaktif bila nama tidak berubah atau tidak valid. */
  canSave: boolean;
  loading: boolean;
  errorMessage?: string;
  /** Pesan sukses di bawah tombol simpan (mis. "Perubahan tersimpan"). */
  savedMessage?: string;
  /** Kartu langganan hanya untuk owner. */
  isOwner: boolean;
  /** User biasa (profil sudah dimuat, bukan owner): tampilkan ajakan "Jadi Owner" ke halaman Subscription. */
  canBecomeOwner?: boolean;
  subscriptionPaid?: boolean;
  /** Keterangan di bawah status, mis. "Aktif hingga 1 Feb 2025". */
  subscriptionDescription?: string;
  /** Foto profil; null = tampilkan ikon. */
  avatarSource?: AvatarSource | null;
  /** Server menyimpan foto untuk user ini (menentukan tombol Hapus Foto). */
  hasAvatar?: boolean;
  /** Unggah atau hapus foto sedang berjalan. */
  avatarBusy?: boolean;
  onPickAvatar?: () => void;
  /** Dipanggil setelah user mengonfirmasi hapus foto. */
  onDeleteAvatar?: () => void;
  /** Email notifikasi tiket menyala (dari server). */
  emailNotifications?: boolean;
  emailNotificationsBusy?: boolean;
  onToggleEmailNotifications?: (enabled: boolean) => void;
  /** Minta tautan buat password baru dikirim ke email akun (item 9). */
  onPressChangePassword?: () => void;
  /** Permintaan tautan sedang berjalan. */
  changePasswordBusy?: boolean;
};

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
  useErrorToast(errorMessage);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Profil"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeTab="profile">
      <View className={isWide ? 'flex-row items-start gap-6' : 'gap-4'}>
        <View className={cn('gap-4', isWide && 'w-80')}>
          {/* Foto profil: hanya terlihat oleh pemiliknya. Tanpa foto, tampil ikon. */}
          <View
            className={cn(
              'items-center gap-3 rounded-card bg-surface',
              isWide ? 'w-80 py-8' : 'py-3.5',
            )}>
            <View className="h-[88px] w-[88px] items-center justify-center overflow-hidden rounded-full bg-dark">
              {avatarSource ? (
                <Image
                  source={avatarSource}
                  style={{ width: 88, height: 88 }}
                  contentFit="cover"
                  accessibilityLabel="Foto profil"
                />
              ) : (
                <PersonIcon size={40} color={AppColors.white} />
              )}
            </View>
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

          {/* Status subscription (owner): link ke halaman Subscription selalu ada, untuk bayar atau memperpanjang. */}
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

          {/* Belum owner: jalan menjadi owner = bayar langganan 1 bulan pertama (halaman Subscription). */}
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

        <View className={cn('gap-4', isWide && 'flex-1')}>
          <FormField
            label="Nama User"
            value={username}
            onChangeText={onChangeUsername}
            placeholder="Nama User"
            maxLength={USERNAME_MAX}
          />
          <FormField label="Email User" value={email} editable={false} placeholder="Email User" />

          {/* Banner verifikasi: hanya saat server mewajibkan verifikasi dan email belum terverifikasi. */}
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

          <AppButton
            label="Simpan Perubahan"
            variant="dark"
            onPress={onSubmit}
            loading={loading}
            disabled={!canSave}
          />
          {savedMessage ? (
            <Text className="text-[13px] font-semibold text-primary">{savedMessage}</Text>
          ) : null}

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
    </ResponsiveScreenLayout>
  );
}
