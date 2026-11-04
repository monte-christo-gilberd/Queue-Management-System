import * as ImagePicker from 'expo-image-picker';
import { useEffect, useState } from 'react';
import { Platform } from 'react-native';

import Profile from '@/components/Profile';
import { useAvatarSource } from '@/hooks/use-avatar-source';
import { deleteAvatar, getMe, setEmailNotifications, updateMe, uploadAvatar, type Me } from '@/lib/users';
import { shrinkToJpeg } from '@/utils/avatar-image';
import { useAuthStore } from '@/store/authStore';
import { useAvatarStore } from '@/store/avatarStore';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';
import {
  describeProfileError,
  subscriptionStatus,
  validateAvatarPick,
  validateUsername,
} from '@/utils/profile';

export default function ProfileRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const [me, setMe] = useState<Me | null>(null);
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [savedMessage, setSavedMessage] = useState('');
  const [avatarBusy, setAvatarBusy] = useState(false);
  const [emailBusy, setEmailBusy] = useState(false);
  const avatarSource = useAvatarSource(me?.avatar_updated_at);

  useEffect(() => {
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

  const trimmed = username.trim();
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
      onMenuItemPress={handleOwnerMenuItemPress}
      username={username}
      email={me?.email ?? ''}
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
      subscriptionPaid={subscription?.active ?? false}
      subscriptionDescription={subscription?.description}
      // Placeholder: belum ada fungsinya sampai alur ganti password lewat email jadi (plan item 9).
    />
  );
}
