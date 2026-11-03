/**
 * Pratinjau satu baris untuk daftar: spasi dan baris baru dipadatkan, dan teks yang melebihi `max` karakter
 * dipotong dengan "…". Catatan lengkap tetap bisa dibuka lewat popup edit.
 */
export function notePreview(note: string, max = 80): string {
  const oneLine = note.replace(/\s+/g, ' ').trim();
  if (oneLine.length <= max) return oneLine;
  return `${oneLine.slice(0, max - 1).trimEnd()}…`;
}
