/**
 * avatar-preview: ukuran foto profil yang diperbesar di popup halaman profil.
 *
 * Dipakai oleh : src/components/Profile.tsx.
 * Bergantung   : tata letak components/ui/ModalCard (padding latar, lebar maksimum kartu, padding mode compact).
 * Catatan      : Foto tersimpan persegi 512x512 (batas OutputSize di backend/internal/imageproc), jadi ukuran sampai
 *                464 px masih tajam. Angka tata letak di bawah harus mengikuti ModalCard `compact`; ubah bersama.
 */

/** Jarak latar gelap ke tepi layar di tiap sisi (`px-[30px]` di ModalCard). */
const BACKDROP_PADDING = 30;
/** Lebar maksimum kartu popup (`max-w-[480px]` di ModalCard). */
const CARD_MAX_WIDTH = 480;
/** Padding di dalam kartu popup di tiap sisi pada mode compact (`p-2` di ModalCard), jadi bingkai foto tipis. */
const CARD_PADDING = 8;
/**
 * Tinggi yang dipakai selain foto: padding atas dan bawah kartu (16) dan ruang aman agar kartu tidak menempel tepi
 * layar (48). Tombol X menumpang di atas foto, jadi tidak menambah tinggi.
 */
const CHROME_HEIGHT = 64;

/**
 * Sisi foto yang diperbesar dalam piksel, dihitung dari ukuran jendela. Fungsi murni.
 * Foto selalu persegi dan muat di dalam kartu popup, baik di layar sempit (mengikuti lebar) maupun layar pendek
 * (mengikuti tinggi). Hasil tidak pernah negatif dan dibulatkan ke bawah.
 * Contoh: (1920, 1080) -> 464; (390, 844) -> 314; (800, 400) -> 336.
 */
export function getAvatarPreviewSize(windowWidth: number, windowHeight: number): number {
  // Lebar kartu mengikuti layar dikurangi jarak latar, tetapi tidak melebihi batas kartu.
  const cardWidth = Math.min(windowWidth - 2 * BACKDROP_PADDING, CARD_MAX_WIDTH);
  // Foto memenuhi lebar bagian dalam kartu.
  const byWidth = cardWidth - 2 * CARD_PADDING;
  // Layar pendek (mis. landscape): sisakan tempat untuk padding kartu dan ruang aman.
  const byHeight = windowHeight - CHROME_HEIGHT;
  // Ambil yang lebih kecil agar foto muat di kedua arah, dan jaga agar tidak negatif.
  return Math.max(0, Math.floor(Math.min(byWidth, byHeight)));
}
