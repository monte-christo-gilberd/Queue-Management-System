import { router } from "expo-router";
import { Pressable, Text, View } from "react-native";

import { useIsWide } from "@/hooks/use-is-wide";
import type { QueueListItem } from "@/types/api";
import { cn } from "@/utils/cn";
import { formatTicketNumber } from "@/utils/format";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { EmptyState } from "@/components/ui/EmptyState";
import { Icon } from "@/components/ui/Icon";
import { useThemeColor } from "@/hooks/use-theme-mode";

export type QueueListFilter = "all" | "active";

export type QueueListProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** Daftar yang sudah difilter oleh developer; komponen render apa adanya. */
  items: QueueListItem[];
  filter: QueueListFilter;
  onChangeFilter: (filter: QueueListFilter) => void;
  loading?: boolean;
  errorMessage?: string;
};

export default function QueueList({
  shell,
  onMenuItemPress,
  items,
  filter,
  onChangeFilter,
  loading = false,
  errorMessage,
}: QueueListProps) {
  const avatarColor = useThemeColor("avatar");
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Semua Antrian"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="queues"
      onBack={() => router.back()}
    >
      <View className="flex-row gap-2.5">
        <Chip
          label="Semua"
          active={filter === "all"}
          onPress={() => onChangeFilter("all")}
        />
        <Chip
          label="Aktif"
          active={filter === "active"}
          onPress={() => onChangeFilter("active")}
        />
      </View>

      {errorMessage ? <Text className="text-[13px] text-live">{errorMessage}</Text> : null}
      {loading ? <Text className="text-[13px] text-muted">Memuat antrian...</Text> : null}
      {!loading && !errorMessage && items.length === 0 ? (
        <EmptyState
          icon={<Icon name="list" size={44} color={avatarColor} />}
          title="Belum Ada Antrian"
          description="Buat antrian baru untuk mulai melayani customer."
        />
      ) : null}

      <View className={isWide ? "flex-row flex-wrap gap-4" : "gap-4"}>
        {items.map(({ queue, currentTicketNumber, counterName }) => {
          const active = queue.status === "active";
          const fg = active ? "text-white" : "text-foreground";
          return (
            <Pressable
              key={queue.id}
              onPress={() =>
                router.push({
                  pathname: "/queues/[queueId]",
                  params: { queueId: queue.id },
                })
              }
              className={cn(
                "min-h-[120px] gap-[3px] rounded-card border p-3.5",
                active ? "border-primary bg-primary" : "border-line",
                isWide && "w-[31.5%]",
              )}
            >
              <Text className={cn("text-[17px] font-extrabold", fg)}>
                {queue.queue_name}
              </Text>
              {/* TODO: konfirmasi sumber data — nomor saat ini & counter tidak ada di objek Queue */}
              <Text className={cn("text-[13px] font-bold", fg)}>
                Nomor Saat Ini:{" "}
                {currentTicketNumber
                  ? formatTicketNumber(currentTicketNumber)
                  : "-"}
              </Text>
              <Text className={cn("text-[12px] font-normal", fg)}>
                {counterName ?? "-"}
              </Text>
            </Pressable>
          );
        })}
      </View>
    </ResponsiveScreenLayout>
  );
}

function Chip({
  label,
  active,
  onPress,
}: {
  label: string;
  active: boolean;
  onPress: () => void;
}) {
  return (
    <Pressable
      onPress={onPress}
      className={cn(
        "rounded-pill px-[22px] py-[9px]",
        active ? "bg-dark" : "bg-surface",
      )}
    >
      <Text
        className={cn(
          "text-[13px] font-bold",
          active ? "text-white" : "text-muted",
        )}
      >
        {label}
      </Text>
    </Pressable>
  );
}
