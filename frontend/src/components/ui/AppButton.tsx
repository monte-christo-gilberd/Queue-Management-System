import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { useThemeColor } from '@/hooks/use-theme-mode';
import { cn } from '@/utils/cn';

type Variant = 'primary' | 'dark' | 'outline' | 'soft' | 'accent' | 'pill-white' | 'pill-dark';

type AppButtonProps = {
  label: string;
  onPress?: () => void;
  variant?: Variant;
  loading?: boolean;
  disabled?: boolean;
  /** Class tambahan; menimpa class dasar yang bertabrakan (mis. `h-9 w-[76px]`). */
  className?: string;
  icon?: React.ReactNode;
};

const BG: Record<Variant, string> = {
  primary: 'bg-primary',
  dark: 'bg-dark',
  outline: 'bg-background',
  soft: 'bg-primarySoft',
  accent: 'bg-accent',
  'pill-white': 'bg-white',
  'pill-dark': 'bg-dark',
};

const FG: Record<Variant, string> = {
  primary: 'text-white',
  dark: 'text-white',
  outline: 'text-foreground',
  soft: 'text-white',
  accent: 'text-white',
  'pill-white': 'text-dark',
  'pill-dark': 'text-white',
};

// Warna spinner adalah prop (bukan style) sehingga tidak bisa lewat className.
const SPINNER: Record<Variant, string> = {
  primary: AppColors.white,
  dark: AppColors.white,
  outline: AppColors.dark,
  soft: AppColors.white,
  accent: AppColors.white,
  'pill-white': AppColors.dark,
  'pill-dark': AppColors.white,
};

export function AppButton({
  label,
  onPress,
  variant = 'dark',
  loading = false,
  disabled = false,
  className,
  icon,
}: AppButtonProps) {
  const isPill = variant === 'pill-white' || variant === 'pill-dark';
  // Tombol outline ada di atas latar tema, jadi spinnernya ikut warna teks tema.
  const foreground = useThemeColor('foreground');
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled || loading}
      className={cn(
        'h-[52px] items-center justify-center rounded-button px-4 active:opacity-70',
        isPill && 'h-7 rounded-pill px-3.5',
        BG[variant],
        variant === 'outline' && 'border border-line',
        // Tombol gelap tetap gelap di dark mode; tepi tipis agar bentuknya terlihat di atas kartu.
        variant === 'dark' && 'dark:border dark:border-line',
        disabled && 'opacity-70',
        className,
      )}>
      {loading ? (
        <ActivityIndicator color={variant === 'outline' ? foreground : SPINNER[variant]} />
      ) : (
        <View className="flex-row items-center gap-2">
          {icon}
          <Text className={cn('text-[16px] font-bold', isPill && 'text-[12px]', FG[variant])}>
            {label}
          </Text>
        </View>
      )}
    </Pressable>
  );
}
