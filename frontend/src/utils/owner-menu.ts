import { router } from 'expo-router';

import type { OwnerMenuItem } from '@/components/layout/ResponsiveScreenLayout';
import { useAuthStore } from '@/store/authStore';

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
    case 'antrian':
      router.navigate('/antrian');
      break;
    case 'riwayat-saya':
      router.navigate('/riwayat');
      break;
    case 'join':
      // dibuka oleh layout sendiri (popup masuk antrian)
      break;
    case 'logout':
      useAuthStore.getState().logout();
      router.replace('/');
      break;
  }
}
