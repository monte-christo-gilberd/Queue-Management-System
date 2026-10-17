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

/** Menyalin teks ke clipboard. Mengembalikan false bila tidak didukung atau ditolak browser. */
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

  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  area.style.position = 'fixed';
  area.style.opacity = '0';
  document.body.appendChild(area);
  area.select();
  const ok = document.execCommand('copy');
  document.body.removeChild(area);
  return ok;
}
