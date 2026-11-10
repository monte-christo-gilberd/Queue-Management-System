/**
 * owner-menu: navigasi menu profil owner, dipakai bersama oleh semua route owner (HP dan desktop).
 *
 * Dipakai oleh : src/app/(owner)/* (dashboard, queues, history, profile, antrian, riwayat, subscription).
 * Bergantung   : expo-router (router), lib/push.ts (releasePushForLogout), store/authStore.ts (logout).
 * Alur         : item 'logout' -> lepas push -> authStore.logout() (membersihkan sesi) -> router.replace('/').
 * Catatan      : Item 'join' tidak dinavigasi di sini karena popup masuk antrian dibuka oleh layout sendiri.
 */

import { router } from 'expo-router';

import type { OwnerMenuItem } from '@/components/layout/ResponsiveScreenLayout';
import { releasePushForLogout } from '@/lib/push';
import { useAuthStore } from '@/store/authStore';

/** Navigasi menu profil — dipakai bersama oleh semua route owner (HP dan desktop). */
export function handleOwnerMenuItemPress(item: OwnerMenuItem) {
  switch (item) {
    case 'create-queue':
      // Dashboard membuka popup buat antrian saat parameter create=1 ada.
      router.navigate({ pathname: '/dashboard', params: { create: '1' } });
      break;
    case 'dashboard':
      router.navigate('/dashboard');
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
      // releasePushForLogout tidak pernah melempar, jadi finally selalu lanjut ke logout walau push gagal dilepas.
      void releasePushForLogout().finally(() => {
        useAuthStore.getState().logout();
        router.replace('/');
      });
      break;
  }
}
