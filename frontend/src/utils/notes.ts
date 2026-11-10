/**
 * notes: pemotong teks catatan owner untuk pratinjau satu baris. Fungsi murni.
 *
 * Dipakai oleh : src/components/QueueDetail.tsx (daftar catatan pada detail antrian).
 * Catatan      : Catatan lengkap disimpan dan diedit lewat endpoint catatan (lihat lib/notes.ts), bukan di sini.
 */

/**
 * Pratinjau satu baris untuk daftar: spasi dan baris baru dipadatkan, dan teks yang melebihi `max` karakter
 * dipotong dengan "…". Catatan lengkap tetap bisa dibuka lewat popup edit.
 */
export function notePreview(note: string, max = 80): string {
  const oneLine = note.replace(/\s+/g, ' ').trim();
  if (oneLine.length <= max) return oneLine;
  return `${oneLine.slice(0, max - 1).trimEnd()}…`;
}
