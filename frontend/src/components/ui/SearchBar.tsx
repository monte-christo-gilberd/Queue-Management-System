import { TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { Icon } from '@/components/ui/Icon';

type SearchBarProps = {
  value: string;
  onChangeText: (value: string) => void;
};

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
