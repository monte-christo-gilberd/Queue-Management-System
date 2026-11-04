/** Aturan password sama dengan backend (`service.ValidatePassword`): 8 karakter sampai 72 byte (batas bcrypt). */
export const PASSWORD_MIN = 8;
export const PASSWORD_MAX_BYTES = 72;

/** Pesan untuk password yang tidak boleh dipakai, atau null bila boleh. Fungsi murni. */
export function validatePassword(password: string): string | null {
  if ([...password].length < PASSWORD_MIN) return `Password minimal ${PASSWORD_MIN} karakter`;
  if (new TextEncoder().encode(password).length > PASSWORD_MAX_BYTES) return `Password terlalu panjang (maksimal ${PASSWORD_MAX_BYTES} byte)`;
  return null;
}
