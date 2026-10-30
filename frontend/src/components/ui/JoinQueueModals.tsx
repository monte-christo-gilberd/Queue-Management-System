import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { ModalCard } from '@/components/ui/ModalCard';

type JoinQueueModalsProps = {
  visible: boolean;
  onClose: () => void;
};

// Pola 13x13 untuk placeholder QR: 3 penanda sudut (5x5) + data semu.
const GRID = 13;
const FINDER = 5;

function isFilled(r: number, c: number): boolean {
  const finders: [number, number][] = [
    [0, 0],
    [0, GRID - FINDER],
    [GRID - FINDER, 0],
  ];
  for (const [fr, fc] of finders) {
    if (r >= fr - 1 && r <= fr + FINDER && c >= fc - 1 && c <= fc + FINDER) {
      const rr = r - fr;
      const cc = c - fc;
      if (rr < 0 || cc < 0 || rr >= FINDER || cc >= FINDER) return false; // jarak kosong di sekeliling
      const ring = rr === 0 || cc === 0 || rr === FINDER - 1 || cc === FINDER - 1;
      const center = rr === 2 && cc === 2;
      return ring || center;
    }
  }
  return (r * 7 + c * 3 + r * c) % 3 === 0;
}

/** Ilustrasi QR code (bukan QR sungguhan). */
function QrPlaceholder() {
  return (
    <View className="rounded-lg bg-white p-3">
      {Array.from({ length: GRID }, (_, r) => (
        <View key={r} className="flex-row">
          {Array.from({ length: GRID }, (_, c) => (
            <View key={c} className={isFilled(r, c) ? 'h-2.5 w-2.5 bg-dark' : 'h-2.5 w-2.5 bg-white'} />
          ))}
        </View>
      ))}
    </View>
  );
}

/**
 * Popup "Masuk ke Antrian": petunjuk memindai QR memakai kamera bawaan HP (tanpa pemindai di dalam
 * aplikasi dan tanpa input tautan manual). QR antrian berisi `<PUBLIC_APP_URL>/join?t=<token>`.
 */
export function JoinQueueModals({ visible, onClose }: JoinQueueModalsProps) {
  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      <Text className="text-[24px] font-extrabold text-foreground">Masuk ke Antrian</Text>

      <View className="h-64 w-full items-center justify-center overflow-hidden rounded-card bg-dark">
        <QrPlaceholder />
        {/* Bingkai pemindai: 4 sudut. */}
        <View className="absolute left-5 top-5 h-7 w-7 rounded-tl-lg border-l-[3px] border-t-[3px] border-white" />
        <View className="absolute right-5 top-5 h-7 w-7 rounded-tr-lg border-r-[3px] border-t-[3px] border-white" />
        <View className="absolute bottom-5 left-5 h-7 w-7 rounded-bl-lg border-b-[3px] border-l-[3px] border-white" />
        <View className="absolute bottom-5 right-5 h-7 w-7 rounded-br-lg border-b-[3px] border-r-[3px] border-white" />
      </View>
      <Text className="text-center text-[12px] text-muted">
        Pindai dengan kamera HP: arahkan kamera bawaan HP ke QR code antrian, lalu buka tautan yang muncul.
      </Text>

      <Pressable onPress={onClose} className="items-center py-1" accessibilityRole="button">
        <Text className="text-[13px] font-bold" style={{ color: AppColors.textMuted }}>
          Tutup
        </Text>
      </Pressable>
    </ModalCard>
  );
}
