/**
 * use-avatar-source: sumber gambar foto profil milik akun ini, dengan cara berbeda di native dan di web.
 *
 * Dipakai oleh : src/app/(user)/profile/index.tsx, src/components/Profile.tsx, src/components/ui/UserAvatar.tsx.
 * Bergantung   : react-native (Platform), store/authStore (token), lib/users (avatarUrl, fetchAvatarBlob).
 * Alur         : updatedAt + token -> native: URI dengan header Authorization. Web: fetchAvatarBlob -> object URL.
 * Catatan      : Platform.OS membedakan dua jalur karena `<img>` di web tidak bisa mengirim header. Object URL dilepas saat berganti atau unmount.
 */

import { useEffect, useMemo, useState } from 'react';
import { Platform } from 'react-native';

import { useAuthStore } from '@/store/authStore';
import { avatarUrl, fetchAvatarBlob } from '@/lib/users';

/** Sumber gambar siap pakai untuk komponen gambar: `uri` wajib, `headers` opsional (hanya native). */
export type AvatarSource = {
  /** URL gambar di native, atau object URL di web. */
  uri: string;
  /** Header permintaan, berisi Authorization di native. Tidak ada di web. */
  headers?: Record<string, string>;
};

/**
 * Sumber gambar foto profil milik sendiri, atau null bila belum ada foto atau masih dimuat.
 * Endpointnya butuh token: di native expo-image mengirim header Authorization sendiri; di web `<img>` tidak
 * bisa, jadi foto diunduh dengan fetch lalu dijadikan object URL (dilepas saat berganti atau unmount).
 * `updatedAt` (avatar_updated_at) memecah cache saat foto diganti.
 *
 * Argumen: `updatedAt` adalah waktu foto terakhir diganti (string ISO). null atau undefined berarti belum ada foto.
 * Mengembalikan `AvatarSource` atau null. Null bila belum ada foto, belum ada token, atau unduhan web belum selesai.
 */
export function useAvatarSource(updatedAt: string | null | undefined): AvatarSource | null {
  const token = useAuthStore((s) => s.token);
  const [webSource, setWebSource] = useState<AvatarSource | null>(null);
  const isWeb = Platform.OS === 'web';

  // Hanya di web: unduh foto dengan token lalu simpan sebagai object URL. Guard `active` membuang hasil yang datang setelah unmount atau setelah updatedAt berganti.
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

  // Native: URI langsung dengan header Authorization, dihitung ulang hanya saat input berubah.
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
