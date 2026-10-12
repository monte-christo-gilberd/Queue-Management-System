import { useEffect, useRef } from 'react';
import { Animated, Easing, Platform, Pressable, StyleSheet, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';

type EyeToggleProps = {
  /** true = password terlihat (mata biasa), false = tersembunyi (mata dicoret). */
  visible: boolean;
  onPress: () => void;
};

const STROKE = 1.5;

/**
 * Ikon mata: kelopak berbentuk lensa + iris bercincin. Saat password disembunyikan,
 * garis coret "/" digambar masuk (scaleX) dengan iris sedikit mengecil; saat terlihat garis ditarik keluar.
 */
export function EyeToggle({ visible, onPress }: EyeToggleProps) {
  const open = useRef(new Animated.Value(visible ? 1 : 0)).current;

  useEffect(() => {
    Animated.timing(open, {
      toValue: visible ? 1 : 0,
      duration: 220,
      easing: Easing.out(Easing.cubic),
      useNativeDriver: Platform.OS !== 'web',
    }).start();
  }, [visible, open]);

  // 0 = terlihat (tanpa coretan), 1 = tersembunyi (coretan penuh)
  const slash = open.interpolate({ inputRange: [0, 1], outputRange: [1, 0] });
  const iris = open.interpolate({ inputRange: [0, 1], outputRange: [0.8, 1] });

  return (
    <Pressable
      onPress={onPress}
      hitSlop={10}
      accessibilityRole="button"
      accessibilityLabel={visible ? 'Sembunyikan password' : 'Tampilkan password'}
      className="h-7 w-7 items-center justify-center">
      {/* Animated.View & transform tidak lewat NativeWind → StyleSheet/style. */}
      <View style={styles.box}>
        {/* Kelopak: persegi dengan 2 sudut berlawanan membulat penuh, diputar 45° → bentuk lensa. */}
        <View style={styles.lidWrap}>
          <View style={styles.lid} />
        </View>
        <Animated.View style={[styles.iris, { transform: [{ scale: iris }] }]} />
        {/* Garis coret: wrapper diputar, isi dianimasikan scaleX. */}
        <View style={styles.slashWrap} pointerEvents="none">
          <Animated.View style={[styles.slash, { transform: [{ scaleX: slash }] }]} />
        </View>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  box: { width: 28, height: 28, alignItems: 'center', justifyContent: 'center' },
  lidWrap: {
    position: 'absolute',
    transform: [{ scaleY: 1.4 }],
  },
  lid: {
    width: 17,
    height: 17,
    borderWidth: STROKE,
    borderColor: AppColors.white,
    borderTopLeftRadius: 17,
    borderBottomRightRadius: 17,
    transform: [{ rotate: '45deg' }],
  },
  iris: {
    width: 10,
    height: 10,
    borderRadius: 5,
    borderWidth: STROKE,
    borderColor: AppColors.white,
  },
  slashWrap: {
    position: 'absolute',
    width: 30,
    alignItems: 'center',
    justifyContent: 'center',
    transform: [{ rotate: '-45deg' }],
  },
  // Tepi warna latar halaman auth memberi celah di sekitar garis seperti pada referensi.
  // Tinggi 3 = garis putih 1 + celah warna latar 1 di atas dan bawah (border-box).
  slash: {
    width: 30,
    height: 3,
    backgroundColor: AppColors.white,
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor: AppColors.dark,
  },
});
