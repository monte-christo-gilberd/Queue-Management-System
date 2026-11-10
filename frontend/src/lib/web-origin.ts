/**
 * web-origin.ts: alamat web untuk tautan join serta penyalinan teks ke clipboard.
 *
 * Dipakai oleh : lib/display-url.ts, components/DisplaySettingsCard.tsx, components/QueueQRCard.tsx.
 * Bergantung   : react-native (Platform), DOM (hanya web).
 * Catatan      : di native, alamat web diambil dari EXPO_PUBLIC_WEB_URL; tanpa itu currentWebOrigin mengembalikan null.
 */
import { Platform } from 'react-native';

/**
 * Alamat web aplikasi untuk membangun tautan join.
 * Web: origin halaman saat ini. Native: EXPO_PUBLIC_WEB_URL (null bila tidak diset).
 */
export function currentWebOrigin(): string | null {
  if (Platform.OS === 'web') {
    return typeof window !== 'undefined' ? window.location.origin : null;
  }
  return process.env.EXPO_PUBLIC_WEB_URL ?? null;
}

/**
 * Menyalin teks ke clipboard. Mengembalikan false bila tidak didukung atau ditolak browser.
 * Hanya web; di native langsung mengembalikan false.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (Platform.OS !== 'web' || typeof document === 'undefined') return false;

  // Clipboard API hanya tersedia di localhost atau HTTPS; di http://<ip-lan> pakai cara lama.
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // lanjut ke fallback
    }
  }

  const previous = document.activeElement as HTMLElement | null;
  // Elemen sementara dipasang DI DALAM dialog yang sedang terbuka (bila ada). Di luarnya, halaman di belakang
  // popup bisa tidak boleh menerima fokus, sehingga select() tidak memilih apa pun dan perintah copy menyalin kosong.
  const host = (previous?.closest('[role="dialog"], [aria-modal="true"]') as HTMLElement | null) ?? document.body;
  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  // 16px mencegah Safari iOS memperbesar halaman saat fokus.
  area.style.cssText = 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;font-size:16px;';
  host.appendChild(area);
  try {
    area.focus({ preventScroll: true });
    area.select();
    // Safari iOS mengabaikan select() pada elemen readonly; rentang eksplisit membuatnya memilih.
    area.setSelectionRange(0, text.length);
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    // Elemen sementara selalu dilepas dan fokus dikembalikan, apa pun hasil salinannya.
    host.removeChild(area);
    previous?.focus?.({ preventScroll: true });
  }
}
