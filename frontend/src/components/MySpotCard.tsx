/**
 * MySpotCard: kartu posisi customer di antrian (nomor sedang dilayani, sebelumnya, dan berikutnya) dan kartu "tidak sedang mengantri".
 *
 * Dipakai oleh : components/QueueDetail.tsx (owner, showNeighbors=false), app/(user)/dashboard.tsx (customer, juga NotQueuingCard)
 * Bergantung   : ui/AppButton, constants/theme (Fonts)
 * Peran        : owner dan customer
 * Catatan      : gaya tampilan sama dengan papan layar live owner; semua nomor sudah diformat oleh pemanggil.
 */
import { Pressable, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { Fonts } from '@/constants/theme';

const MONO = Fonts?.mono;

/** Props kartu posisi customer di antrian. */
export type MySpotCardProps = {
  /** Nama antrian; ditampilkan dalam huruf kecil di kepala kartu. */
  queueName: string;
  /** Nomor yang sedang dilayani, sudah diformat (mis. "A-042"); null bila belum ada. */
  currentNumber: string | null;
  /** Nomor sebelumnya dan berikutnya, sudah diformat; null bila tidak ada. */
  previousNumber: string | null;
    /** Nomor berikutnya, sudah diformat; null bila tidak ada. */
  nextNumber: string | null;
  /** false = sembunyikan baris sebelumnya/berikutnya (mis. tampilan owner di detail antrian). */
  showNeighbors?: boolean;
  /** Ketuk kartu (mis. buka daftar tiket). Tanpa ini kartu tidak bisa ditekan. */
  onPress?: () => void;
  /** Teks petunjuk di bawah kartu bila `onPress` ada. */
  hint?: string;
};

/** Kartu antrian aktif (customer). Gaya sama dengan papan layar live owner. */
export function MySpotCard({
  queueName,
  currentNumber,
  previousNumber,
  nextNumber,
  showNeighbors = true,
  onPress,
  hint = 'ketuk untuk daftar tiket ▾',
}: MySpotCardProps) {
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      className="w-full gap-7 rounded-[20px] bg-board px-8 py-7 dark:border dark:border-line active:opacity-80"
    >
      <View className="flex-row items-start justify-between">
        <Text className="text-[16px] tracking-[1px] text-boardMuted" style={{ fontFamily: MONO }}>
          {queueName.toLowerCase()}
        </Text>
        <View className="flex-row items-center gap-2">
          <View className="h-2.5 w-2.5 rounded-full bg-liveGreen" />
          <Text className="text-[14px] text-liveGreen" style={{ fontFamily: MONO }}>live</Text>
        </View>
      </View>

      <View className="h-px bg-boardLine" />

      <View className="items-center gap-2">
        <Text className="text-[12px] text-boardDim" style={{ fontFamily: MONO }}>sedang dilayani</Text>
        <Text className="text-[64px] leading-[72px] tracking-[2px] text-white" style={{ fontFamily: MONO }}>
          {currentNumber ?? '---'}
        </Text>
      </View>

      {showNeighbors ? (
        <>
          <View className="h-px bg-boardLine" />

          <View className="flex-row items-start justify-between">
            <View className="gap-1">
              <Text className="text-[14px] text-boardDim" style={{ fontFamily: MONO }}>sebelumnya</Text>
              <Text className="text-[20px] text-boardMuted" style={{ fontFamily: MONO }}>{previousNumber ?? '-'}</Text>
            </View>
            <View className="items-end gap-1">
              <Text className="text-[14px] text-boardDim" style={{ fontFamily: MONO }}>berikutnya</Text>
              <Text className="text-[20px] text-boardMuted" style={{ fontFamily: MONO }}>{nextNumber ?? '-'}</Text>
            </View>
          </View>
        </>
      ) : null}

      {onPress ? (
        <Text className="text-center text-[12px] text-boardDim" style={{ fontFamily: MONO }}>
          {hint}
        </Text>
      ) : null}
    </Pressable>
  );
}

/** Props kartu "tidak sedang mengantri". */
export type NotQueuingCardProps = {
  /** Tombol masuk antrian. Hanya diberikan untuk layar lebar (PC); di HP tersedia di navbar bawah. */
  onJoin?: () => void;
};

/** Ditampilkan di beranda ketika customer tidak sedang mengantri. */
export function NotQueuingCard({ onJoin }: NotQueuingCardProps) {
  return (
    <View className="items-center gap-2 rounded-card bg-surface p-5">
      <Text className="text-[15px] font-extrabold text-foreground">Anda tidak sedang mengantri</Text>
      <Text className="text-center text-[12px] font-normal text-muted">
        Masukkan token dari penyelenggara untuk masuk ke antrian.
      </Text>
      {onJoin ? <AppButton label="Masuk Ke Antrian" variant="dark" className="mt-2 self-stretch" onPress={onJoin} /> : null}
    </View>
  );
}
