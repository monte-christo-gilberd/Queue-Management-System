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
  if (Platform.OS !== 'web' || typeof navigator === 'undefined' || !navigator.clipboard) return false;
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
