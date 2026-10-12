import { Stack } from 'expo-router';

// TODO(developer): tambahkan auth guard di sini (redirect ke /login kalau belum ada token).
export default function OwnerLayout() {
  return <Stack screenOptions={{ headerShown: false }} />;
}
