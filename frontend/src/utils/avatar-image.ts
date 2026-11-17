/**
 * avatar-image: memperkecil foto profil di browser sebelum diunggah. Berjalan hanya di web.
 *
 * Dipakai oleh : src/app/(user)/profile/index.tsx.
 * Bergantung   : API browser (createImageBitmap, document, canvas). Tidak ada di native.
 * Catatan      : Dipanggil setelah validateAvatarPick (utils/profile.ts) lolos; bila hasilnya null, pemanggil
 *                mengunggah berkas asli.
 */

/**
 * Web: memperkecil foto di browser sebelum diunggah (crop tengah persegi, 512x512, JPEG). Browser sudah
 * memutar foto ponsel sesuai orientasi EXIF saat menggambar ke canvas, dan unggahan jadi jauh lebih kecil.
 * Server tetap memvalidasi dan mengolah ulang, jadi ini hanya optimasi. Mengembalikan null bila gagal
 * (pemanggil lalu mengunggah berkas asli).
 */
export async function shrinkToJpeg(file: Blob, size = 512): Promise<Blob | null> {
  // Lingkungan tanpa DOM atau tanpa createImageBitmap (native) langsung mengembalikan null.
  if (typeof document === 'undefined' || typeof createImageBitmap !== 'function') return null;
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
    const side = Math.min(bitmap.width, bitmap.height);
    const out = Math.min(side, size);
    const canvas = document.createElement('canvas');
    canvas.width = out;
    canvas.height = out;
    const ctx = canvas.getContext('2d');
    if (!ctx) return null;
    ctx.fillStyle = '#fff'; // JPEG tanpa alpha: bagian transparan jadi putih
    ctx.fillRect(0, 0, out, out);
    ctx.drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, out, out);
    bitmap.close();
    return await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.85));
  } catch {
    return null;
  }
}
