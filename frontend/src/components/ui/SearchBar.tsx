/**
 * SearchBar: kotak pencarian dengan ikon kaca pembesar di kiri.
 *
 * Dipakai oleh : belum ada pemakai (tidak ditemukan impor ke komponen ini)
 * Bergantung   : constants/app-theme, components/ui/Icon
 * Catatan      : Placeholder tertulis tetap ("Insert Username / Email") dan tidak lewat props.
 */
import { TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { Icon } from '@/components/ui/Icon';

type SearchBarProps = {
  /** Teks pencarian yang sedang diketik. Dikontrol induk. */
  value: string;
  /** Dipanggil setiap teks berubah, dengan teks terbaru. */
  onChangeText: (value: string) => void;
};

/** Kotak pencarian terkontrol; huruf kapital otomatis dimatikan. */
export function SearchBar({ value, onChangeText }: SearchBarProps) {
  return (
    <View className="h-12 flex-row items-center gap-2 rounded-card bg-surface px-3.5">
      <Icon name="search" size={20} color={AppColors.textMuted} />
      <TextInput
        value={value}
        onChangeText={onChangeText}
        placeholder="Insert Username / Email"
        placeholderTextColor={AppColors.placeholder}
        autoCapitalize="none"
        className="flex-1 text-[14px] text-foreground"
      />
    </View>
  );
}
