import { Redirect } from 'expo-router';

// Halaman "Edit Profil" kini menjadi halaman Profil di /profile — route lama diarahkan ke sana.
export default function ProfileEditRedirect() {
  return <Redirect href="/profile" />;
}
