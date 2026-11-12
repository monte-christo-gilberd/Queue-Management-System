/**
 * useAuthKeyboardNav: navigasi keyboard untuk halaman auth (web saja).
 *
 * Dipakai oleh : components/LoginScreen.tsx, components/RegisterScreen.tsx
 * Bergantung   : react (useEffect), react-native (Platform)
 * Catatan      : `items` berurutan dari atas ke bawah. Panah atas/bawah memindahkan fokus ke elemen berikutnya yang
 *                tersedia (elemen null atau aria-disabled dilewati). Enter pada elemen yang bukan field teks menekannya.
 *                Enter di field teks tidak ditangani di sini; itu lewat onSubmitEditing supaya berlaku juga di native.
 *                Listener di fase capture karena Pressable RNW memanggil stopPropagation pada keydown.
 */
import { useEffect, type RefObject } from 'react';
import { Platform } from 'react-native';

export function useAuthKeyboardNav(items: RefObject<unknown>[]) {
  useEffect(() => {
    if (Platform.OS !== 'web') return;

    const onKeyDown = (e: KeyboardEvent) => {
      const active = document.activeElement;
      // Di web, ref menunjuk ke elemen DOM; elemen aktif bisa berupa elemen itu sendiri atau turunannya.
      const index = items.findIndex((r) => {
        const el = r.current as Element | null;
        return !!el && (el === active || el.contains(active));
      });
      if (index === -1) return;

      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const step = e.key === 'ArrowDown' ? 1 : -1;
        for (let i = index + step; i >= 0 && i < items.length; i += step) {
          const el = items[i].current as HTMLElement | null;
          if (!el || el.getAttribute('aria-disabled') === 'true') continue;
          el.focus();
          return;
        }
        return;
      }

      const isTextField = active instanceof HTMLInputElement || active instanceof HTMLTextAreaElement;
      if (e.key === 'Enter' && !isTextField) {
        const el = items[index].current as HTMLElement | null;
        if (!el || el.getAttribute('aria-disabled') === 'true') return;
        e.preventDefault();
        el.click();
      }
    };

    document.addEventListener('keydown', onKeyDown, true);
    return () => document.removeEventListener('keydown', onKeyDown, true);
  }, [items]);
}
