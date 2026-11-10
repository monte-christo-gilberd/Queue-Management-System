/**
 * QrCode: gambar QR code dari teks (biasanya tautan join) memakai SVG yang dihitung di perangkat.
 *
 * Dipakai oleh : components/QueueQRCard.tsx
 * Bergantung   : lib/qr-matrix (buildQrMatrix, buildQrPath), react-native-svg
 * Catatan      : Tidak ada request gambar ke server, dan token tidak ikut terkirim sebagai URL gambar.
 */
import { useMemo } from 'react';
import Svg, { Path, Rect } from 'react-native-svg';

import { buildQrMatrix, buildQrPath } from '@/lib/qr-matrix';

type QrCodeProps = {
  /** Teks yang dikodekan, mis. tautan join. Setiap perubahan menghitung ulang QR. */
  value: string;
  /** Perkiraan sisi QR dalam piksel. Hasil dibulatkan ke kelipatan jumlah modul. Default 240. */
  size?: number;
};

/** QR digambar di perangkat dari tautan (SVG): tanpa request gambar dan tanpa token di URL. */
export function QrCode({ value, size = 240 }: QrCodeProps) {
  // Matriks dihitung ulang hanya saat value berubah.
  const { d, viewBox } = useMemo(() => buildQrPath(buildQrMatrix(value)), [value]);
  // Kelipatan bulat dari piksel per modul agar tepi modul tajam dan mudah dipindai.
  const px = Math.max(1, Math.floor(size / viewBox)) * viewBox;
  return (
    <Svg width={px} height={px} viewBox={`0 0 ${viewBox} ${viewBox}`} accessibilityRole="image">
      <Rect width={viewBox} height={viewBox} fill="#ffffff" />
      <Path d={d} fill="#000000" />
    </Svg>
  );
}
