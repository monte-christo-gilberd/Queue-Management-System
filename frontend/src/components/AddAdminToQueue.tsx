import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { useIsWide } from '@/hooks/use-is-wide';
import { cn } from '@/utils/cn';
import type { AdminContact, Queue } from '@/types/api';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
  type ShellProps,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { Avatar } from '@/components/ui/Avatar';

export type AddAdminToQueueProps = {
  shell?: ShellProps;
  onMenuItemPress?: (item: OwnerMenuItem) => void;
  queue?: Queue | null;
  /** Admin yang belum aktif di queue mana pun. TODO: backend belum mendukung ini (queue_admins) */
  admins: AdminContact[];
  onAdd: (admin: AdminContact) => void;
};

export default function AddAdminToQueue({
  shell,
  onMenuItemPress,
  queue,
  admins,
  onAdd,
}: AddAdminToQueueProps) {
  const isWide = useIsWide();

  return (
    <ResponsiveScreenLayout
      title="Add Admin to Queue"
      userName={shell?.userName}
      onMenuItemPress={onMenuItemPress}
      activeItem="admin-contact"
      activeTab="admins"
      onBack={() => router.back()}>
      <View className="flex-row items-center justify-between rounded-card bg-primary p-4">
        <View>
          <Text className="text-[11px] text-white">Nama Antrian Aktif</Text>
          <Text className="text-[17px] font-extrabold text-white">{queue?.queue_name ?? '-'}</Text>
        </View>
        <AppButton label="Aktif" variant="pill-white" />
      </View>

      <View className={isWide ? 'flex-row flex-wrap gap-4' : 'gap-4'}>
        {admins.map((admin) => (
          <View
            key={admin.id}
            className={cn(
              'flex-row items-center gap-3 rounded-card border border-line p-3.5',
              isWide && 'w-[48.5%]',
            )}>
            <Avatar />
            <View className="flex-1 gap-0.5">
              <Text className="text-[15px] font-extrabold text-foreground">{admin.name}</Text>
              <Text className="text-[12px] font-normal text-muted">{admin.email}</Text>
            </View>
            <AppButton
              label="Add"
              variant="dark"
              className="h-9 w-[84px] px-0"
              onPress={() => onAdd(admin)}
            />
          </View>
        ))}
      </View>

      <Text className="text-[12px] font-normal text-muted">Not Active in Any Queue</Text>
    </ResponsiveScreenLayout>
  );
}
