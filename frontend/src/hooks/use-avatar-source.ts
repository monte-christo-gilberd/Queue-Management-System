import { useEffect, useMemo, useState } from 'react';
import { Platform } from 'react-native';

import { useAuthStore } from '@/store/authStore';
import { avatarUrl, fetchAvatarBlob } from '@/lib/users';

export type AvatarSource = { uri: string; headers?: Record<string, string> };

/**
 * Sumber gambar foto profil milik sendiri, atau null bila belum ada foto atau masih dimuat.
 * Endpointnya butuh token: di native expo-image mengirim header Authorization sendiri; di web `<img>` tidak
 * bisa, jadi foto diunduh dengan fetch lalu dijadikan object URL (dilepas saat berganti atau unmount).
 * `updatedAt` (avatar_updated_at) memecah cache saat foto diganti.
 */
export function useAvatarSource(updatedAt: string | null | undefined): AvatarSource | null {
  const token = useAuthStore((s) => s.token);
  const [webSource, setWebSource] = useState<AvatarSource | null>(null);
  const isWeb = Platform.OS === 'web';

  useEffect(() => {
    if (!isWeb || !updatedAt || !token) return;
    let active = true;
    let objectUrl: string | null = null;
    fetchAvatarBlob().then(
      (blob) => {
        if (!active) return;
        objectUrl = URL.createObjectURL(blob);
        setWebSource({ uri: objectUrl });
      },
      () => {
        if (active) setWebSource(null);
      },
    );
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [isWeb, updatedAt, token]);

  const nativeSource = useMemo<AvatarSource | null>(
    () =>
      !isWeb && updatedAt && token
        ? { uri: avatarUrl(updatedAt), headers: { Authorization: `Bearer ${token}` } }
        : null,
    [isWeb, updatedAt, token],
  );

  if (!updatedAt || !token) return null;
  return isWeb ? webSource : nativeSource;
}
