import { router } from "expo-router";
import { Text, View } from "react-native";

import { AppColors } from "@/constants/app-theme";
import { useIsWide } from "@/hooks/use-is-wide";
import type { AdminContact as AdminContactType } from "@/types/api";
import { cn } from "@/utils/cn";
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from "@/components/layout/ResponsiveScreenLayout";
import { AppButton } from "@/components/ui/AppButton";
import { Avatar } from "@/components/ui/Avatar";
import { EmptyState } from "@/components/ui/EmptyState";
import { Icon } from "@/components/ui/Icon";
import { SearchBar } from "@/components/ui/SearchBar";

export type AdminContactProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  /** TODO: backend belum mendukung ini (belum ada endpoint queue_admins) */
  admins: AdminContactType[];
  search: string;
  onChangeSearch: (value: string) => void;
};

export default function AdminContact({
  shell,
  onMenuItemPress,
  admins,
  search,
  onChangeSearch,
}: AdminContactProps) {
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Admin Contact"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="admin-contact"
      onBack={() => router.back()}
    >
      <SearchBar value={search} onChangeText={onChangeSearch} />

      {admins.length === 0 ? (
        <EmptyState
          icon={<Icon name="users" size={44} color={AppColors.avatar} />}
          title="Tidak Ada Admin Terdaftar"
          description="Tambahkan admin untuk mulai mengelola antrian"
        />
      ) : (
        <View className={isWide ? "flex-row flex-wrap gap-4" : "gap-4"}>
          {admins.map((admin) => {
            const active = !!admin.activeQueueName;
            return (
              <View
                key={admin.id}
                className={cn(
                  "flex-row items-center gap-3 rounded-card border p-3.5",
                  active ? "border-primary bg-primary" : "border-line",
                  isWide && "w-[48.5%]",
                )}
              >
                <Avatar active={active} />
                <View className="flex-1 gap-0.5">
                  <Text
                    className={cn(
                      "text-[15px] font-extrabold",
                      active ? "text-white" : "text-foreground",
                    )}
                  >
                    {admin.name}
                  </Text>
                  <Text
                    className={cn(
                      "text-[12px] font-normal",
                      active ? "text-white" : "text-muted",
                    )}
                  >
                    {admin.email}
                  </Text>
                </View>
                <View
                  className={cn(
                    "rounded-pill px-3 py-[7px]",
                    active ? "bg-white" : "bg-surface",
                  )}
                >
                  <Text
                    className={cn(
                      "text-[10px] font-extrabold",
                      // Badge putih (aktif) bertema tetap; badge surface mengikuti tema.
                      active ? "text-textMuted" : "text-muted",
                    )}
                  >
                    {active ? "Aktif di Queue" : "Not Active in Any Queue"}
                  </Text>
                </View>
              </View>
            );
          })}
        </View>
      )}

      <AppButton
        label="Add User"
        variant="dark"
        className={isWide ? "w-60 self-start" : undefined}
        icon={<Icon name="user-add" size={22} color={AppColors.white} />}
        onPress={() => router.push("/admins/add")}
      />
    </ResponsiveScreenLayout>
  );
}
