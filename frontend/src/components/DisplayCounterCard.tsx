/**
 * DisplayCounterCard: kartu satu counter di layar publik antrian.
 *
 * Dipakai oleh : components/DisplayScreen.tsx
 * Bergantung   : react-native (Text, View)
 * Peran        : publik
 * Catatan      : komponen presentasi murni tanpa interaksi; ukuran dan sorotan dihitung oleh induknya.
 */
import { Text, View } from 'react-native';

/** Props kartu satu counter di layar publik. */
export type DisplayCounterCardProps = {
  /** Nama counter; null = jangan tampilkan (antrian dengan satu counter). */
  title: string | null;
  /** Nomor yang sedang dilayani ("A-007"); null = counter sedang tidak melayani. */
  number: string | null;
  /** Lebar kartu dalam px (dihitung dari jumlah kolom). */
  width: number;
  /** Ukuran huruf nomor dalam px (dihitung dari jumlah kartu dan ukuran layar). */
  numberFontSize: number;
  /** Nomor baru saja berganti: kartu disorot sesaat. */
  highlighted?: boolean;
  /** Tinggi minimal kartu dalam px. */
  minHeight: number;
};

/**
 * Kartu satu counter di layar publik. Bentuknya mengikuti kartu "Nomor Anda" di layar customer
 * (kotak berwarna utama, label kecil, nomor sangat besar). Counter yang tidak melayani tetap tampil, redup, supaya
 * tidak terbaca seperti counter yang dihapus.
 */
export function DisplayCounterCard({ title, number, width, numberFontSize, highlighted = false, minHeight }: DisplayCounterCardProps) {
  const idle = number === null;
  return (
    <View
      testID="display-card"
      accessibilityLabel={title ? `Counter ${title}` : 'Counter'}
      className="items-center justify-center gap-2 rounded-card bg-primary p-4"
      style={{
        width,
        minHeight,
        opacity: idle ? 0.35 : 1,
        borderWidth: 6,
        borderColor: highlighted ? '#FFFFFF' : 'transparent',
      }}
    >
      {title ? <Text className="text-center text-[28px] font-extrabold text-white">{title}</Text> : null}
      <Text className="text-center text-[18px] font-semibold text-white">
        {idle ? 'Menunggu panggilan' : 'Sedang dilayani'}
      </Text>
      <Text className="text-center font-black text-white" style={{ fontSize: numberFontSize, lineHeight: numberFontSize * 1.1 }}>
        {number ?? '-'}
      </Text>
    </View>
  );
}
