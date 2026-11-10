/**
 * Icon: ikon glyph Unicode (Icon) dan ikon yang digambar dengan View (PersonIcon, LockIcon, ScanIcon, AtIcon).
 *
 * Dipakai oleh : banyak layar dan komponen, mis. components/layout/ResponsiveScreenLayout.tsx,
 *                components/ui/SearchBar.tsx, components/ui/UserAvatar.tsx, components/LoginScreen.tsx
 * Catatan      : Glyph Unicode masih placeholder (lihat TODO di GLYPHS). expo-symbols sudah ada di package.json.
 */
import { Text, View } from 'react-native';

/** Nama glyph yang didukung Icon. Setiap nama wajib punya entri di GLYPHS (dijaga tipe Record). */
export type IconName =
  | 'back'
  | 'home'
  | 'list'
  | 'users'
  | 'history'
  | 'search'
  | 'refresh'
  | 'clock'
  | 'check'
  | 'user-add'
  | 'person';

// TODO: ganti glyph ini dengan icon set asli (mis. @expo/vector-icons / expo-symbols) agar mirip mockup.
const GLYPHS: Record<IconName, string> = {
  person: '◉',
  back: '‹',
  home: '⌂',
  list: '☰',
  users: '⚇',
  history: '↺',
  search: '⌕',
  refresh: '↻',
  clock: '◷',
  check: '✓',
  'user-add': '+',
};

type IconProps = {
  /** Nama glyph dari IconName. */
  name: IconName;
  /** Ukuran font dalam piksel; tinggi baris ikut 1.1 kali nilai ini. Default 24. */
  size?: number;
  /** Warna glyph. Default hitam. */
  color?: string;
};

/** Ikon glyph Unicode dari GLYPHS, dirender sebagai teks di tengah. */
export function Icon({ name, size = 24, color = '#000' }: IconProps) {
  return (
    <Text style={{ fontSize: size, color, lineHeight: size * 1.1, textAlign: 'center' }}>
      {GLYPHS[name]}
    </Text>
  );
}

type PersonIconProps = {
  /** Diameter area ikon dalam piksel. Default 24. */
  size?: number;
  /** Warna garis ikon. Default putih. */
  color?: string;
};

/** Ikon orang (kepala + bahu) yang digambar dengan View. */
export function PersonIcon({ size = 24, color = '#fff' }: PersonIconProps) {
  const head = size * 0.42;
  const stroke = Math.max(2, size * 0.09);
  return (
    <View className="items-center justify-center"
      // Ukuran dinamis (prop `size`) tidak bisa jadi class statis.
      style={{ width: size, height: size }}>
      <View
        style={{
          width: head,
          height: head,
          borderRadius: head / 2,
          borderWidth: stroke,
          borderColor: color,
        }}
      />
      <View
        style={{
          width: size * 0.84,
          height: size * 0.36,
          marginTop: size * 0.06,
          borderTopLeftRadius: size * 0.42,
          borderTopRightRadius: size * 0.42,
          borderWidth: stroke,
          borderBottomWidth: 0,
          borderColor: color,
        }}
      />
    </View>
  );
}

type SmallIconProps = {
  /** Ukuran ikon dalam piksel. Default berbeda per ikon (20 atau 24). */
  size?: number;
  /** Warna ikon. Default putih. */
  color?: string;
};

/** Ikon gembok (badan + gagang) yang digambar dengan View. */
export function LockIcon({ size = 20, color = '#fff' }: SmallIconProps) {
  // Ketebalan garis minimal 1.5 piksel agar tetap terlihat di ukuran kecil.
  const stroke = Math.max(1.5, size * 0.09);
  return (
    <View className="items-center justify-center"
      // Ukuran dinamis (prop `size`) tidak bisa jadi class statis.
      style={{ width: size, height: size }}>
      <View
        style={{
          width: size * 0.42,
          height: size * 0.34,
          borderTopLeftRadius: size * 0.21,
          borderTopRightRadius: size * 0.21,
          borderWidth: stroke,
          borderBottomWidth: 0,
          borderColor: color,
        }}
      />
      <View
        style={{
          width: size * 0.7,
          height: size * 0.46,
          borderRadius: size * 0.08,
          borderWidth: stroke,
          borderColor: color,
        }}
      />
    </View>
  );
}

/** Ikon "@" untuk field email. */
export function AtIcon({ size = 20, color = '#fff' }: SmallIconProps) {
  return (
    <Text style={{ fontSize: size, color, lineHeight: size * 1.1, textAlign: 'center' }}>@</Text>
  );
}

/** Ikon scan/QR: empat sudut bingkai + kotak kecil di tengah (digambar dengan View). */
export function ScanIcon({ size = 24, color = '#fff' }: SmallIconProps) {
  const arm = size * 0.36;
  const stroke = Math.max(2, size * 0.09);
  // Helper: satu sudut bingkai dengan ukuran dan border yang sama; extra menentukan posisi dan sisi yang digambar.
  const corner = (extra: object) => ({
    position: 'absolute' as const,
    width: arm,
    height: arm,
    borderColor: color,
    ...extra,
  });
  return (
    <View style={{ width: size, height: size }}>
      <View style={corner({ top: 0, left: 0, borderTopWidth: stroke, borderLeftWidth: stroke, borderTopLeftRadius: stroke })} />
      <View style={corner({ top: 0, right: 0, borderTopWidth: stroke, borderRightWidth: stroke, borderTopRightRadius: stroke })} />
      <View style={corner({ bottom: 0, left: 0, borderBottomWidth: stroke, borderLeftWidth: stroke, borderBottomLeftRadius: stroke })} />
      <View style={corner({ bottom: 0, right: 0, borderBottomWidth: stroke, borderRightWidth: stroke, borderBottomRightRadius: stroke })} />
      <View
        className="absolute rounded-[2px]"
        style={{
          width: size * 0.26,
          height: size * 0.26,
          left: size * 0.37,
          top: size * 0.37,
          backgroundColor: color,
        }}
      />
    </View>
  );
}
