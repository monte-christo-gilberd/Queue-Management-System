import { Pressable, Text, View } from 'react-native';

import { cn } from '@/utils/cn';

type CurrentNumberCardProps = {
  /** Nomor tampilan, mis. "A-042". */
  numberLabel: string;
  /** Nama counter yang memanggil; null bila counter default disembunyikan (jangan ditampilkan). */
  counterName?: string | null;
  onPressNote?: () => void;
  /** Titik merah "live" di pojok kanan atas. */
  showLiveDot?: boolean;
  /** Versi kompak (layar Detail Antrian) vs versi besar (Dashboard). */
  compact?: boolean;
};

export function CurrentNumberCard({
  numberLabel,
  counterName,
  onPressNote,
  showLiveDot = false,
  compact = false,
}: CurrentNumberCardProps) {
  return (
    <View className={cn('rounded-card bg-primary p-4', compact && 'items-center py-3')}>
      {compact ? null : <Text className="text-[12px] font-semibold text-white">Nomor Saat Ini</Text>}
      {showLiveDot ? (
        <View className="absolute right-4 top-3.5 h-2.5 w-2.5 rounded-full bg-live" />
      ) : null}
      <View className={cn('flex-row items-center justify-between', compact && 'justify-center gap-3')}>
        <Text className={cn('text-[56px] font-black text-white', compact && 'text-[44px]')}>
          {numberLabel}
        </Text>
        <Pressable
          onPress={onPressNote}
          className={cn(
            'h-16 w-16 items-center justify-center rounded-2xl bg-black',
            compact && 'h-14 w-14',
          )}>
          <View className="h-[30px] w-[30px] rounded-[4px] rounded-tr-xl bg-white" />
        </Pressable>
      </View>
      {counterName ? (
        <Text className={cn('text-[13px] text-white', compact && 'text-[12px]')}>{`Counter yang Dituju: ${counterName}`}</Text>
      ) : null}
    </View>
  );
}
