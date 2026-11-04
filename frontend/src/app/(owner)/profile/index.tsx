import { useEffect, useState } from 'react';

import Profile from '@/components/Profile';
import { getMe, updateMe, type Me } from '@/lib/users';
import { useAuthStore } from '@/store/authStore';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';
import { describeProfileError, subscriptionStatus, validateUsername } from '@/utils/profile';

export default function ProfileRoute() {
  const displayName = useAuthStore((s) => s.displayName);
  const [me, setMe] = useState<Me | null>(null);
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [savedMessage, setSavedMessage] = useState('');

  useEffect(() => {
    let active = true;
    getMe().then(
      (data) => {
        if (!active) return;
        setMe(data);
        setUsername(data.username);
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
      isOwner={me?.is_owner === true}
      subscriptionPaid={subscription?.active ?? false}
      subscriptionDescription={subscription?.description}
      // Placeholder: belum ada fungsinya sampai alur ganti password lewat email jadi (plan item 9).
    />
  );
}
