/**
 * google-auth.ts: konfigurasi client ID Google per platform dan keputusan apakah tombol Google ditampilkan.
 *
 * Dipakai oleh : components/auth/GoogleSignInSection.tsx, hooks/use-google-sign-in.ts.
 * Bergantung   : tidak ada (fungsi murni; dapat diuji di Node).
 * Catatan      : EXPO_PUBLIC_* dibakar ke bundle saat build dan hanya terbaca bila ditulis statis
 *                (`process.env.EXPO_PUBLIC_X`), jadi GOOGLE_CLIENT_IDS di bawah tidak boleh disusun dinamis.
 *                Client ID bukan rahasia. Tanpa client ID untuk platform yang sedang berjalan, tombol Google
 *                disembunyikan (pemanggil hook provider tanpa client ID akan melempar galat).
 */

/** GoogleClientIds: client ID OAuth per platform. Kosong/undefined = platform itu tidak dikonfigurasi. */
export type GoogleClientIds = {
  web?: string;
  android?: string;
  ios?: string;
};

/** Platform yang dikenal (nilai `Platform.OS` React Native). */
export type GooglePlatform = 'web' | 'android' | 'ios' | string;

/** clean membuang spasi; string kosong dianggap tidak diisi. */
function clean(value: string | undefined): string | undefined {
  const v = value?.trim();
  return v ? v : undefined;
}

/** GOOGLE_CLIENT_IDS: dibaca sekali saat modul dimuat dari EXPO_PUBLIC_GOOGLE_*_CLIENT_ID (ditulis statis agar terbakar). */
export const GOOGLE_CLIENT_IDS: GoogleClientIds = {
  web: clean(process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID),
  android: clean(process.env.EXPO_PUBLIC_GOOGLE_ANDROID_CLIENT_ID),
  ios: clean(process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID),
};

/** clientIdFor mengembalikan client ID untuk platform, atau undefined bila tidak dikonfigurasi atau platform tak dikenal. */
export function clientIdFor(platform: GooglePlatform, ids: GoogleClientIds = GOOGLE_CLIENT_IDS): string | undefined {
  if (platform === 'web' || platform === 'android' || platform === 'ios') return clean(ids[platform]);
  return undefined;
}

/** isGoogleEnabled: true bila platform yang berjalan punya client ID, jadi tombol Google aman ditampilkan. */
export function isGoogleEnabled(platform: GooglePlatform, ids: GoogleClientIds = GOOGLE_CLIENT_IDS): boolean {
  return clientIdFor(platform, ids) !== undefined;
}
