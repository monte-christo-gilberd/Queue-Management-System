import { Text, TextInput, View, type TextInputProps } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { cn } from '@/utils/cn';

type FormFieldProps = TextInputProps & {
  label?: string;
  /** Gaya kotak: "outline" (border) atau "filled" (abu-abu). */
  tone?: 'outline' | 'filled';
};

export function FormField({ label, tone = 'filled', className, ...inputProps }: FormFieldProps) {
  return (
    <View className="gap-1.5">
      {label ? <Text className="text-[14px] font-bold text-foreground">{label}</Text> : null}
      <TextInput
        placeholderTextColor={AppColors.placeholder}
        {...inputProps}
        className={cn(
          'h-12 rounded-input px-3.5 text-[15px] font-semibold text-foreground',
          tone === 'outline' ? 'border border-line bg-white' : 'bg-surface',
          className,
        )}
      />
    </View>
  );
}
