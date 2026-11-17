/**
 * EmptyState: kartu tengah untuk daftar atau layar yang belum berisi data.
 *
 * Dipakai oleh : app/(user)/riwayat.tsx, app/(user)/antrian.tsx, components/History.tsx
 * Catatan      : Tanpa aksi. Ikon dirender apa adanya, jadi pemanggil menentukan gambarnya.
 */
import { Text, View } from 'react-native';

type EmptyStateProps = {
  /** Elemen di atas judul, mis. komponen Icon. Dirender apa adanya. */
  icon: React.ReactNode;
  /** Judul tebal satu baris. */
  title: string;
  /** Penjelasan singkat di bawah judul, mis. langkah berikutnya. */
  description: string;
};

/** Kartu kosong berisi ikon, judul, dan deskripsi di tengah. */
export function EmptyState({ icon, title, description }: EmptyStateProps) {
  return (
    <View className="items-center gap-2 rounded-card bg-surface px-6 py-14">
      {icon}
      <Text className="text-center text-[16px] font-extrabold text-foreground">{title}</Text>
      <Text className="text-center text-[12px] text-muted">{description}</Text>
    </View>
  );
}
