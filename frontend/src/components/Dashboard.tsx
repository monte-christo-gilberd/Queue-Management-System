import { Text, View } from "react-native";

import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";

export type DashboardProps = {
  shell?: ShellProps;
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

export default function Dashboard({
  shell,
  onMenuItemPress,
  boards,
  hasActiveQueue = false,
  errorMessage,
  topCard,
  canCreateQueue = true,
}: DashboardProps) {
  return (
    <ResponsiveScreenLayout
      title="Dashboard"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="dashboard"
      activeTab="home"
    >
      {topCard}

      {errorMessage ? (
        <Text className="text-[12px] font-normal text-live">{errorMessage}</Text>
      ) : null}

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
