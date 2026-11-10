/**
 * password: validasi password di sisi klien (register dan reset password).
 *
 * Dipakai oleh : src/app/register.tsx, src/app/reset-password.tsx.
 * Catatan      : Aturan sama dengan backend (service.ValidatePassword). Validasi ulang di server tetap berlaku.
 */

/** Aturan password sama dengan backend (`service.ValidatePassword`): 8 karakter sampai 72 byte (batas bcrypt). */
export const PASSWORD_MIN = 8;
/** Batas ukuran password dalam byte UTF-8 (bukan jumlah karakter), sesuai batas bcrypt. */
export const PASSWORD_MAX_BYTES = 72;

/**
 * Pesan untuk password yang tidak boleh dipakai, atau null bila boleh. Fungsi murni.
 * Contoh: "abc" -> "Password minimal 8 karakter"; delapan huruf "ä" (16 byte) -> null.
 */
export function validatePassword(password: string): string | null {
  // Jumlah karakter dihitung per code point, jadi emoji atau huruf beraksen dihitung satu.
  if ([...password].length < PASSWORD_MIN) return `Password minimal ${PASSWORD_MIN} karakter`;
  if (new TextEncoder().encode(password).length > PASSWORD_MAX_BYTES) return `Password terlalu panjang (maksimal ${PASSWORD_MAX_BYTES} byte)`;
  return null;
}
