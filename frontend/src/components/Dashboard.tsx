/**
 * Dashboard: kerangka halaman dashboard yang dipakai owner dan customer.
 *
 * Dipakai oleh : app/(owner)/dashboard.tsx
 * Bergantung   : components/layout/ResponsiveScreenLayout, components/ui/Toast (useErrorToast)
 * Peran        : owner dan customer
 * Catatan      : komponen ini hanya pembungkus layout dan state kosong; logika utama (memuat antrian, memilih peran) ada di route.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { Text, View } from "react-native";

import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";

export type DashboardProps = {
  /** Konteks shell layout; saat ini hanya `userName` yang dipakai untuk header. Opsional. */
  shell?: ShellProps;
  /** Dipanggil saat item menu owner ditekan; opsional, tanpa nilai menu tidak ada aksi. */
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** Papan antrian owner (satu kartu per antrian aktif terbaru). Dirender oleh route. */
  boards?: React.ReactNode;
  /** true bila ada antrian aktif yang ditampilkan lewat `boards`. */
  hasActiveQueue?: boolean;
  /** Pesan error daftar antrian, ditampilkan di atas papan. */
  errorMessage?: string;
  /** Kartu di atas papan: kondisi antrian saya (customer). */
  topCard?: React.ReactNode;
  /** Owner: true (default). Customer: false, sehingga bagian kosong owner disembunyikan. */
  canCreateQueue?: boolean;
};

/** Menampilkan papan antrian, atau pesan kosong yang sesuai peran bila tidak ada antrian aktif. */
export default function Dashboard({
  shell,
  onMenuItemPress,
  boards,
  hasActiveQueue = false,
  errorMessage,
  topCard,
  canCreateQueue = true,
}: DashboardProps) {
  // Pesan error ditampilkan sebagai toast, bukan teks di dalam layout.
  useErrorToast(errorMessage);
  // Customer (canCreateQueue=false) tanpa antrian aktif tidak melihat apa pun di area papan.
  return (
    <ResponsiveScreenLayout
      title="Dashboard"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="dashboard"
      activeTab="home"
    >
      {topCard}

      {hasActiveQueue ? (
        <View className="w-full gap-4">{boards}</View>
      ) : canCreateQueue ? (
        <View className="items-center gap-2 rounded-card bg-surface p-5">
          <View className="mb-1 flex-row gap-2.5">
            <View className="h-[30px] w-[30px] rounded-full bg-live" />
            <View className="h-[30px] w-[30px] rounded-full bg-avatar" />
            <View className="h-[30px] w-[30px] rounded-full bg-avatar" />
          </View>
          <Text className="text-[15px] font-extrabold text-foreground">
            Belum ada antrian yang aktif
          </Text>
          <Text className="text-center text-[12px] font-normal text-muted">
            Buat antrian baru atau masuk ke antrian yang tersedia.
          </Text>
        </View>
      ) : null}
    </ResponsiveScreenLayout>
  );
}
