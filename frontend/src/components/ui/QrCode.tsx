import { useMemo } from 'react';
import Svg, { Path, Rect } from 'react-native-svg';

import { buildQrMatrix, buildQrPath } from '@/lib/qr-matrix';

type QrCodeProps = {
  /** Teks yang dikodekan, mis. tautan join. */
  value: string;
  size?: number;
};

/** QR digambar di perangkat dari tautan (SVG): tanpa request gambar dan tanpa token di URL. */
export function QrCode({ value, size = 240 }: QrCodeProps) {
  const { d, viewBox } = useMemo(() => buildQrPath(buildQrMatrix(value)), [value]);
  return (
    <Svg width={size} height={size} viewBox={`0 0 ${viewBox} ${viewBox}`} accessibilityRole="image">
      <Rect width={viewBox} height={viewBox} fill="#ffffff" />
      <Path d={d} fill="#000000" />
    </Svg>
  );
}
