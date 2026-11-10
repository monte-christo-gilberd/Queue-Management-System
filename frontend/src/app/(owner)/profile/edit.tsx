/**
 * profile/edit: stub: route lama Edit Profil, hanya meneruskan ke /profile.
 *
 * Rute        : /profile/edit.
 * Akses       : butuh login (AuthGate dari (owner)/_layout.tsx).
 * Dipakai oleh: tautan atau bookmark lama.
 * Bergantung  : Redirect dari expo-router.
 * Catatan     : STUB. Halaman edit profil sudah digabung ke /profile.
 */
import { Redirect } from 'expo-router';

/**
 * Stub: halaman Edit Profil sudah digabung ke halaman Profil di /profile, jadi route lama hanya diarahkan ke sana.
 */
export default function ProfileEditRedirect() {
  return <Redirect href="/profile" />;
}
