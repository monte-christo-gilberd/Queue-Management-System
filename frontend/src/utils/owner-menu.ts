import { router } from 'expo-router';

import type { OwnerMenuItem } from '@/components/layout/ResponsiveScreenLayout';

/** Navigasi menu profil — dipakai bersama oleh semua route owner (HP dan desktop). */
export function handleOwnerMenuItemPress(item: OwnerMenuItem) {
  switch (item) {
    case 'dashboard':
      router.navigate('/dashboard');
      break;
    case 'admin-contact':
      router.navigate('/admins');
      break;
    case 'queues':
      router.navigate('/queues');
      break;
    case 'history':
      router.navigate('/history');
      break;
    case 'profile':
      router.push('/profile');
      break;
    case 'settings':
      // TODO: layar Pengaturan belum ada di mockup.
      break;
    case 'logout':
      // TODO(developer): hapus token sebelum redirect.
      router.replace('/');
      break;
  }
}
