/**
 * FormField: input berlabel untuk form owner (kotak input dengan label di atasnya).
 *
 * Dipakai oleh : components/SkipSettingsCard.tsx, components/CreateQueueModal.tsx, components/Profile.tsx
 * Bergantung   : constants/app-theme, utils/cn
 * Catatan      : Prop TextInput lain diteruskan apa adanya ke TextInput.
 */
import { Text, TextInput, View, type TextInputProps } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { cn } from '@/utils/cn';

type FormFieldProps = TextInputProps & {
  /** Teks label di atas kotak. Bila kosong, label tidak dirender. */
  label?: string;
  /** Gaya kotak: "outline" (border) atau "filled" (abu-abu). Default "filled". */
  tone?: 'outline' | 'filled';
};

/** Label opsional di atas TextInput dengan gaya kotak yang dipilih lewat tone. */
export function FormField({ label, tone = 'filled', className, ...inputProps }: FormFieldProps) {
  return (
    <View className="gap-1.5">
      {label ? <Text className="text-[14px] font-bold text-foreground">{label}</Text> : null}
      <TextInput
        placeholderTextColor={AppColors.placeholder}
        {...inputProps}
        // className dari pemanggil ditaruh paling akhir supaya bisa menimpa gaya dasar.
        className={cn(
          'h-12 rounded-input px-3.5 text-[15px] font-semibold text-foreground',
          tone === 'outline' ? 'border border-line bg-white' : 'bg-surface',
          className,
        )}
      />
    </View>
  );
}
