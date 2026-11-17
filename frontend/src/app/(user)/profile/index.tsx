/**
 * profile/index: profil pengguna: nama, foto, notifikasi email, verifikasi email, ganti password, dan status langganan.
 *
 * Rute        : /profile.
 * Akses       : butuh login (AuthGate dari (user)/_layout.tsx).
 * Dipakai oleh: menu panel (case profile di utils/menu).
 * Bergantung  : Profile, getMe/updateMe/uploadAvatar (lib/users), useAvatarSource, useAvatarStore, lib/email-verification, lib/password-reset.
 * Catatan     : Bagian langganan hanya untuk owner. Calon owner (belum pernah bayar) melihat tawaran menjadi owner.
 */
import * as ImagePicker from 'expo-image-picker';
import { useEffect, useState } from 'react';
import { AppState, Platform } from 'react-native';

import Profile from '@/components/Profile';
import { useAvatarSource } from '@/hooks/use-avatar-source';
import { describeResendError, onEmailVerified, resendSuccessMessage, resendVerification } from '@/lib/email-verification';
import { describeForgotError, requestPasswordReset } from '@/lib/password-reset';
import { deleteAvatar, getMe, setEmailNotifications, updateMe, uploadAvatar, type Me } from '@/lib/users';
import { shrinkToJpeg } from '@/utils/avatar-image';
import { useAuthStore } from '@/store/authStore';
import { useAvatarStore } from '@/store/avatarStore';
import { handleMenuItemPress } from '@/utils/menu';
import {
  describeProfileError,
  subscriptionStatus,
  validateAvatarPick,
  validateUsername,
} from '@/utils/profile';

/**
 * Profil: memuat data pengguna saat layar dibuka, lalu mengelola nama pengguna, foto, notifikasi email, dan aksi akun.
 * Setiap aksi punya penanda busy sendiri agar tombolnya terkunci selama permintaan berjalan.
 */
export default function ProfileRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const [me, setMe] = useState<Me | null>(null);
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [savedMessage, setSavedMessage] = useState('');
  const [passwordBusy, setPasswordBusy] = useState(false);
  const [avatarBusy, setAvatarBusy] = useState(false);
  const [emailBusy, setEmailBusy] = useState(false);
  const [resendBusy, setResendBusy] = useState(false);
  const avatarSource = useAvatarSource(me?.avatar_updated_at);

  useEffect(() => {
    // Hasil yang datang setelah layar ditutup diabaikan agar tidak mengubah state.
    let active = true;
    getMe().then(
      (data) => {
        if (!active) return;
        setMe(data);
        setUsername(data.username);
        useAvatarStore.getState().setUpdatedAt(data.avatar_updated_at);
      },
      (e) => {
        if (active) setErrorMessage(describeProfileError(e));
      },
    );
    return () => {
      active = false;
    };
  }, []);

  // Muat ulang status saat email baru diverifikasi di tab lain, atau saat tab ini kembali aktif, agar banner dan
  // tombol Jadi Owner langsung berubah tanpa refresh manual.
  useEffect(() => {
    let active = true;
    const refresh = () => {
      getMe().then(
        (data) => {
          if (active) setMe((prev) => (prev && prev.email_verified === data.email_verified && prev.is_owner === data.is_owner ? prev : data));
        },
        () => {},
      );
    };
    const stopBroadcast = onEmailVerified(refresh);
    const appSub = AppState.addEventListener('change', (state) => {
      if (state === 'active') refresh();
    });
    const onVisible = () => {
      if (document.visibilityState === 'visible') refresh();
    };
    if (Platform.OS === 'web') document.addEventListener('visibilitychange', onVisible);
    return () => {
      active = false;
      stopBroadcast();
      appSub.remove();
      if (Platform.OS === 'web') document.removeEventListener('visibilitychange', onVisible);
    };
  }, []);

  const trimmed = username.trim();
  // Simpan hanya bila nama berubah dan lolos validasi; sebelum profil dimuat, tombol tetap nonaktif.
  const canSave = me !== null && trimmed !== me.username && validateUsername(username) === null;
  const subscription = me?.is_owner ? subscriptionStatus(me.subscription_expires_at) : null;

  async function save() {
    const invalid = validateUsername(username);
    if (invalid) {
      setErrorMessage(invalid);
      return;
    }
    setLoading(true);
    setErrorMessage('');
    setSavedMessage('');
    try {
      const updated = await updateMe(trimmed);
      setMe(updated);
      setUsername(updated.username);
      useAuthStore.getState().setDisplayName(updated.username);
      setSavedMessage('Perubahan tersimpan');
    } catch (e) {
      setErrorMessage(describeProfileError(e));
    } finally {
      setLoading(false);
    }
  }

  async function pickAvatar() {
    setErrorMessage('');
    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ['images'],
      allowsEditing: true,
      aspect: [1, 1],
      quality: 0.8,
    });
    if (result.canceled || !result.assets?.[0]) return;
    const asset = result.assets[0];

    const invalid = validateAvatarPick(asset.fileSize, asset.mimeType);
    if (invalid) {
      setErrorMessage(invalid);
      return;
    }
    setAvatarBusy(true);
    try {
      let updated: Me;
      if (Platform.OS === 'web' && asset.file) {
        // Web: perkecil di browser dulu (hemat unggahan, orientasi EXIF benar); gagal berarti kirim aslinya.
        const small = await shrinkToJpeg(asset.file);
        updated = small ? await uploadAvatar(small, 'avatar.jpg') : await uploadAvatar(asset.file, asset.fileName ?? 'avatar');
      } else {
        updated = await uploadAvatar({
          uri: asset.uri,
          name: asset.fileName ?? 'avatar.jpg',
          type: asset.mimeType ?? 'image/jpeg',
        });
      }
      setMe(updated);
      useAvatarStore.getState().setUpdatedAt(updated.avatar_updated_at);
    } catch (e) {
      setErrorMessage(describeProfileError(e));
    } finally {
      setAvatarBusy(false);
    }
  }

  async function requestPasswordChange() {
    if (!me?.email) return;
    setErrorMessage('');
    setSavedMessage('');
    setPasswordBusy(true);
    try {
      await requestPasswordReset(me.email);
      setSavedMessage(`Tautan untuk membuat password baru sudah dikirim ke ${me.email}`);
    } catch (e) {
      setErrorMessage(describeForgotError(e));
    } finally {
      setPasswordBusy(false);
    }
  }

  async function resendVerificationEmail() {
    if (!me?.email) return;
    setErrorMessage('');
    setSavedMessage('');
    setResendBusy(true);
    try {
      await resendVerification();
      setSavedMessage(resendSuccessMessage(me.email));
    } catch (e) {
      setErrorMessage(describeResendError(e));
    } finally {
      setResendBusy(false);
    }
  }

  async function toggleEmail(enabled: boolean) {
    setErrorMessage('');
    setEmailBusy(true);
    try {
      setMe(await setEmailNotifications(enabled));
    } catch (e) {
      setErrorMessage(describeProfileError(e));
    } finally {
      setEmailBusy(false);
    }
  }

  async function removeAvatar() {
    setErrorMessage('');
    setAvatarBusy(true);
    try {
      await deleteAvatar();
      setMe((prev) => (prev ? { ...prev, avatar_updated_at: null } : prev));
      useAvatarStore.getState().setUpdatedAt(null);
    } catch (e) {
      setErrorMessage(describeProfileError(e));
    } finally {
      setAvatarBusy(false);
    }
  }

  return (
    <Profile
      shell={{ userName: displayName ?? undefined }}
      onMenuItemPress={handleMenuItemPress}
      username={username}
      email={me?.email ?? ''}
      // Tampilkan banner hanya setelah profil dimuat dan server menyatakan belum terverifikasi.
      emailVerified={me === null ? true : me.email_verified}
      onPressResendVerification={() => void resendVerificationEmail()}
      resendVerificationBusy={resendBusy}
      onChangeUsername={(value) => {
        setUsername(value);
        setSavedMessage('');
      }}
      onSubmit={() => void save()}
      canSave={canSave}
      loading={loading}
      errorMessage={errorMessage}
      savedMessage={savedMessage}
      avatarSource={avatarSource}
      hasAvatar={me?.avatar_updated_at != null}
      avatarBusy={avatarBusy}
      onPickAvatar={() => void pickAvatar()}
      onDeleteAvatar={() => void removeAvatar()}
      emailNotifications={me?.email_notifications ?? true}
      emailNotificationsBusy={emailBusy || me === null}
      onToggleEmailNotifications={(v) => void toggleEmail(v)}
      isOwner={me?.is_owner === true}
      canBecomeOwner={me !== null && !me.is_owner}
      subscriptionPaid={subscription?.active ?? false}
      subscriptionDescription={subscription?.description}
      onPressChangePassword={() => void requestPasswordChange()}
      changePasswordBusy={passwordBusy || me === null}
    />
  );
}
