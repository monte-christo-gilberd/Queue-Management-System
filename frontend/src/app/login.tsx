/**
 * login: stub: route lama /login yang hanya mengarahkan ke /.
 *
 * Rute        : /login (sudah tidak punya UI).
 * Akses       : publik.
 * Dipakai oleh: tautan atau bookmark lama.
 * Bergantung  : Redirect dari expo-router.
 * Catatan     : STUB. Halaman login yang sebenarnya ada di /.
 */
import { Redirect } from 'expo-router';

/**
 * Stub: login sekarang ada di "/", jadi route lama /login hanya meneruskan ke sana.
 */
export default function LoginRedirect() {
  // Redirect (bukan salinan UI) supaya hanya ada satu halaman login.
  return <Redirect href="/" />;
}
