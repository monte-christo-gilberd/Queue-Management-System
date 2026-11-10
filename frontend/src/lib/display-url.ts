/**
 * display-url.ts: membangun URL layar publik dari token layar.
 *
 * Dipakai oleh : components/DisplaySettingsCard.tsx.
 * Bergantung   : lib/web-origin.ts (currentWebOrigin).
 */
import { currentWebOrigin } from './web-origin';

/**
 * Alamat layar publik untuk token ini (null bila origin web tidak diketahui, mis. native tanpa EXPO_PUBLIC_WEB_URL).
 * Terpisah dari `display.ts` karena `web-origin` memuat react-native, yang tidak bisa dimuat tes unit di Node.
 */
export function displayUrl(token: string): string | null {
  const origin = currentWebOrigin();
  return origin ? `${origin}/display/${encodeURIComponent(token)}` : null;
}
