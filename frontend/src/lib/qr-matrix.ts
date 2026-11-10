/**
 * qr-matrix.ts: membuat matriks dan path SVG untuk QR dari teks (mis. tautan join).
 *
 * Dipakai oleh : components/QueueQRCard.tsx, components/ui/QrCode.tsx.
 * Bergantung   : paket `qrcode` (create). renderQrPngDataUrl memakai `document` dan hanya berjalan di web.
 */
import { create } from 'qrcode';

/** Zona tenang standar di sekeliling QR (dalam modul). */
export const QR_QUIET_ZONE = 4;

/** QrMatrix: matriks modul QR. `size` adalah jumlah modul per sisi tanpa zona tenang; `isDark` true untuk modul gelap, indeks mulai dari 0. */
export type QrMatrix = {
  /** Jumlah modul per sisi, tanpa zona tenang. */
  size: number;
  /** true bila modul di (row, col) gelap; indeks mulai dari 0. */
  isDark: (row: number, col: number) => boolean;
};

/** Matriks QR dari teks (mis. tautan join). Level `L`: tautan join panjang, jadi modul dibuat sesedikit dan sebesar mungkin agar mudah dipindai. Layar bersih, jadi koreksi galat tinggi tidak perlu. Fungsi murni: tanpa DOM, jalan di web dan native. */
export function buildQrMatrix(text: string): QrMatrix {
  const { modules } = create(text, { errorCorrectionLevel: 'L' });
  return { size: modules.size, isDark: (row, col) => modules.get(row, col) === 1 };
}

/**
 * Satu string path SVG untuk semua modul gelap (garis horizontal per baris berurutan),
 * dengan zona tenang. Satu elemen `Path` jauh lebih ringan daripada ratusan `Rect`.
 * `viewBox` = size + 2 x quietZone, dipakai sebagai viewBox SVG.
 */
export function buildQrPath(matrix: QrMatrix, quietZone: number = QR_QUIET_ZONE): { d: string; viewBox: number } {
  const { size } = matrix;
  const parts: string[] = [];
  for (let row = 0; row < size; row++) {
    let col = 0;
    while (col < size) {
      if (!matrix.isDark(row, col)) {
        col++;
        continue;
      }
      const start = col;
      while (col < size && matrix.isDark(row, col)) col++;
      parts.push(`M${start + quietZone} ${row + quietZone}h${col - start}v1h-${col - start}z`);
    }
  }
  return { d: parts.join(''), viewBox: size + quietZone * 2 };
}

/** PNG data URL dari tautan, via canvas. Hanya web (butuh `document`); null di tempat lain. */
export function renderQrPngDataUrl(text: string, pixels: number = 960): string | null {
  if (typeof document === 'undefined') return null;
  const matrix = buildQrMatrix(text);
  const total = matrix.size + QR_QUIET_ZONE * 2;
  // Skala per modul agar sisi gambar kira-kira `pixels` piksel; minimal 1 piksel per modul.
  const scale = Math.max(1, Math.floor(pixels / total));
  const canvas = document.createElement('canvas');
  canvas.width = total * scale;
  canvas.height = total * scale;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;
  // Latar putih dan zona tenang wajib agar pemindai mendapat kontras yang cukup.
  ctx.fillStyle = '#ffffff';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#000000';
  for (let row = 0; row < matrix.size; row++) {
    for (let col = 0; col < matrix.size; col++) {
      if (matrix.isDark(row, col)) {
        ctx.fillRect((col + QR_QUIET_ZONE) * scale, (row + QR_QUIET_ZONE) * scale, scale, scale);
      }
    }
  }
  return canvas.toDataURL('image/png');
}
