/**
 * confirm-password: pesan galat konfirmasi password untuk layar daftar dan reset password.
 *
 * Dipakai oleh : src/app/register.tsx, src/app/reset-password.tsx.
 * Catatan      : Pesan hanya muncul setelah pengguna selesai mengisi field konfirmasi (blur) atau menekan tombol kirim,
 *                bukan sambil mengetik. Karakter pertama di field konfirmasi selalu berbeda dari password yang lebih
 *                panjang, jadi pengecekan langsung membuat pesan muncul sebelum pengguna sempat menyelesaikan ketikan.
 */

/** Pesan di bawah field konfirmasi ketika isinya berbeda dari password. */
export const CONFIRM_PASSWORD_MISMATCH = 'Password tidak sama';

/**
 * Pesan galat konfirmasi password, atau undefined bila tidak ada yang perlu ditampilkan. Fungsi murni.
 * `touched` bernilai true setelah field konfirmasi kehilangan fokus atau tombol kirim ditekan.
 * Contoh: ('abcdefgh', 'abc', false) -> undefined (masih mengetik); ('abcdefgh', 'abc', true) -> 'Password tidak sama';
 * ('abcdefgh', 'abcdefgh', true) -> undefined; ('abcdefgh', '', true) -> undefined (field kosong ditangani validasi kirim).
 */
export function getConfirmPasswordError(
  password: string,
  confirmPassword: string,
  touched: boolean,
): string | undefined {
  // Pengguna belum selesai mengisi: tidak menampilkan apa pun sambil mengetik.
  if (!touched) return undefined;
  // Field kosong ditangani validasi saat kirim, dan password yang sama berarti tidak ada galat.
  if (!confirmPassword || confirmPassword === password) return undefined;
  // Isi berbeda dari password: tampilkan pesan.
  return CONFIRM_PASSWORD_MISMATCH;
}
