import { useState } from 'react';
import { Platform, StyleSheet, TextInput, View, type TextInputProps } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { cn } from '@/utils/cn';
import { EyeToggle } from '@/components/auth/EyeToggle';

type AuthInputProps = TextInputProps & {
  icon: React.ReactNode;
  /** Tampilkan tombol mata di ujung kanan untuk membuka/menutup isi password. */
  showPasswordToggle?: boolean;
};

/** Input outline tipis tanpa latar solid dengan ikon di kiri (gaya halaman auth). */
export function AuthInput({ icon, className, showPasswordToggle = false, ...inputProps }: AuthInputProps) {
  // State tampilan murni: password sedang terlihat atau tidak.
  const [passwordVisible, setPasswordVisible] = useState(false);

  return (
    <View className="h-12 flex-row items-center gap-3 px-3.5">
      {/* Border semi-transparan: lapisan terpisah supaya teks input tetap 100% putih. */}
      <View
        className="absolute bottom-0 left-0 right-0 top-0 rounded-[6px] border border-white opacity-[0.45]"
        pointerEvents="none"
      />
      <View className="w-5 items-center">{icon}</View>
      <TextInput
        placeholderTextColor={AppColors.placeholder}
        selectionColor={AppColors.white}
        {...inputProps}
        secureTextEntry={showPasswordToggle ? !passwordVisible : inputProps.secureTextEntry}
        className={cn(
          'h-full flex-1 bg-transparent text-[13px] tracking-[0.5px] text-white',
          className,
        )}
        style={styles.input}
      />
      {showPasswordToggle ? (
        <EyeToggle visible={passwordVisible} onPress={() => setPasswordVisible((v) => !v)} />
      ) : null}
    </View>
  );
}

// outlineStyle hanya ada di web dan bukan utility Tailwind-RN → tetap StyleSheet.
const styles = StyleSheet.create({
  input: Platform.OS === 'web' ? ({ outlineStyle: 'none' } as object) : {},
});
