/**
 * menu: navigasi item menu panel profil dan navbar, dipakai bersama oleh semua route grup (user) (HP dan desktop).
 *
 * Dipakai oleh : src/app/(user)/* (dashboard, queues, history, profile, antrian, riwayat, subscription).
 * Bergantung   : expo-router (router), lib/push-channel (releasePushForLogout: Web Push di web, token FCM di Android), store/authStore.ts (logout).
 * Alur         : item 'logout' -> lepas push -> authStore.logout() (membersihkan sesi) -> router.replace('/login').
 * Catatan      : Item 'join' tidak dinavigasi di sini karena popup masuk antrian dibuka oleh layout sendiri.
 */

import { router } from 'expo-router';

import type { MenuItem } from '@/components/layout/ResponsiveScreenLayout';
import { releasePushForLogout } from '@/lib/push-channel';
import { useAuthStore } from '@/store/authStore';

/** Navigasi menu panel profil, dipakai bersama oleh semua route grup (user) (HP dan desktop). Item khusus owner (queues, history) hanya tampil bila isOwner. */
export function handleMenuItemPress(item: MenuItem) {
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
        router.replace('/login');
      });
      break;
  }
}
